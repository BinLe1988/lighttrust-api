package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
)

type ReconcileRequestsInput struct {
	ConfigID      int64
	AccountID     string
	Regions       []string
	ChannelIDs    []int
	Period        reconcile.Period
	MaturityDelay time.Duration
	Now           time.Time
}

type ReconcileRequestCounters struct {
	Internal int                          `json:"internal"`
	Upstream int                          `json:"upstream"`
	ByStatus map[reconcile.ItemStatus]int `json:"by_status"`
}

func ReconcileRequests(input ReconcileRequestsInput) (ReconcileRequestCounters, error) {
	if err := input.Period.Validate(); err != nil {
		return ReconcileRequestCounters{}, err
	}
	if input.ConfigID == 0 {
		return ReconcileRequestCounters{}, fmt.Errorf("reconciliation config id is required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now()
	}
	if input.MaturityDelay <= 0 {
		input.MaturityDelay = 30 * time.Minute
	}

	internalLogs, err := model.FindInternalLogsForReconcile(
		input.ChannelIDs,
		input.Period.Start.Unix(),
		input.Period.End.Unix(),
	)
	if err != nil {
		return ReconcileRequestCounters{}, err
	}
	upstreamRecords, err := model.FindUpstreamInvocationsForReconcile(
		input.AccountID,
		input.Regions,
		input.Period.Start.Unix(),
		input.Period.End.Unix(),
	)
	if err != nil {
		return ReconcileRequestCounters{}, err
	}

	internal := make([]reconcile.InternalInvocation, 0, len(internalLogs))
	for _, log := range internalLogs {
		upstreamModel := log.ModelName
		other, _ := common.StrToMap(log.Other)
		if value, ok := other["upstream_model_name"].(string); ok && value != "" {
			upstreamModel = value
		}
		internal = append(internal, reconcile.InternalInvocation{
			RequestID:         log.RequestID,
			UpstreamRequestID: log.UpstreamRequestID,
			ChannelID:         log.ChannelID,
			Timestamp:         time.Unix(log.CreatedAt, 0),
			ModelID:           upstreamModel,
			InputTokens:       int64(log.PromptTokens),
			OutputTokens:      int64(log.CompletionTokens),
		})
	}
	upstream := make([]reconcile.Invocation, 0, len(upstreamRecords))
	for _, record := range upstreamRecords {
		upstream = append(upstream, reconcile.Invocation{
			Provider:              record.Provider,
			AccountID:             record.AccountID,
			Region:                record.Region,
			RequestID:             record.RequestID,
			LocalRequestID:        record.LocalRequestID,
			ChannelID:             record.ChannelID,
			Timestamp:             time.Unix(record.InvokedAt, 0),
			Operation:             record.Operation,
			ModelID:               record.ModelID,
			NormalizedModelID:     record.NormalizedModelID,
			ServiceTier:           record.ServiceTier,
			RoutingType:           record.RoutingType,
			InputTokens:           record.InputTokens,
			OutputTokens:          record.OutputTokens,
			CacheReadInputTokens:  record.CacheReadInputTokens,
			CacheWriteInputTokens: record.CacheWriteInputTokens,
		})
	}

	results := reconcile.NewMatcher(2*time.Minute).Match(internal, upstream)
	counters := ReconcileRequestCounters{
		Internal: len(internal),
		Upstream: len(upstream),
		ByStatus: make(map[reconcile.ItemStatus]int),
	}
	for _, result := range results {
		item := buildReconcileItem(input, result, internal, upstream, upstreamRecords)
		if err := model.UpsertReconcileItem(&item); err != nil {
			return counters, err
		}
		counters.ByStatus[reconcile.ItemStatus(item.Status)]++
	}
	return counters, nil
}

func buildReconcileItem(
	input ReconcileRequestsInput,
	result reconcile.MatchResult,
	internal []reconcile.InternalInvocation,
	upstream []reconcile.Invocation,
	upstreamRecords []model.UpstreamInvocation,
) model.ReconcileItem {
	item := model.ReconcileItem{
		ConfigID:    input.ConfigID,
		MatchMethod: string(result.Method),
		Confidence:  string(result.Confidence),
		Status:      string(result.Status),
		Maturity:    string(reconcile.MaturityFinal),
		Currency:    "USD",
	}
	identity := ""
	observedAt := time.Time{}
	if result.InternalIndex >= 0 {
		value := internal[result.InternalIndex]
		item.InternalRequestID = value.RequestID
		item.InternalModelID = value.ModelID
		item.InternalInputTokens = value.InputTokens
		item.InternalOutputTokens = value.OutputTokens
		identity += "internal:" + value.RequestID
		observedAt = value.Timestamp
	}
	if result.UpstreamIndex >= 0 {
		value := upstream[result.UpstreamIndex]
		item.UpstreamInvocationID = upstreamRecords[result.UpstreamIndex].ID
		item.UpstreamModelID = value.ModelID
		item.UpstreamInputTokens = value.InputTokens
		item.UpstreamOutputTokens = value.OutputTokens
		item.UpstreamCacheReadTokens = value.CacheReadInputTokens
		item.UpstreamCacheWriteTokens = value.CacheWriteInputTokens
		identity += "|upstream:" + value.RequestID
		if observedAt.IsZero() || value.Timestamp.After(observedAt) {
			observedAt = value.Timestamp
		}
	}
	if (result.Status == reconcile.ItemStatusInternalMissing || result.Status == reconcile.ItemStatusUpstreamMissing) &&
		input.Now.Sub(observedAt) < input.MaturityDelay {
		item.Status = string(reconcile.ItemStatusPending)
		item.Maturity = string(reconcile.MaturityPending)
	}
	item.ItemKey = fmt.Sprintf("%x", common.Sha256Raw([]byte(fmt.Sprintf("%d|%s", input.ConfigID, identity))))
	return item
}

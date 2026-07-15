package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/shopspring/decimal"
)

type CostIngestionCounters struct {
	Scanned  int `json:"scanned"`
	Upserted int `json:"upserted"`
}

func IngestReconcileCosts(
	ctx context.Context,
	provider reconcile.CostProvider,
	period reconcile.Period,
	runID string,
) (CostIngestionCounters, error) {
	if provider == nil {
		return CostIngestionCounters{}, errors.New("reconciliation cost provider is nil")
	}
	if err := period.Validate(); err != nil {
		return CostIngestionCounters{}, err
	}
	if runID == "" {
		return CostIngestionCounters{}, errors.New("reconciliation run id is required")
	}

	buckets, err := provider.PullDailyCosts(ctx, period)
	if err != nil {
		return CostIngestionCounters{}, err
	}
	counters := CostIngestionCounters{Scanned: len(buckets)}
	for _, bucket := range buckets {
		if err := bucket.Validate(); err != nil {
			return counters, err
		}
		record := model.UpstreamCostBucket{
			SourceKey:      bucket.SourceKey,
			Provider:       bucket.Provider,
			AccountID:      bucket.AccountID,
			PeriodStart:    bucket.Period.Start.Unix(),
			PeriodEnd:      bucket.Period.End.Unix(),
			Region:         bucket.Region,
			ModelID:        bucket.ModelID,
			Operation:      bucket.Operation,
			UsageType:      bucket.UsageType,
			TokenCategory:  bucket.TokenCategory,
			ServiceTier:    bucket.ServiceTier,
			RoutingType:    bucket.RoutingType,
			UsageQuantity:  bucket.UsageQuantity,
			UnblendedCost:  bucket.UnblendedCost,
			NetCost:        bucket.NetCost,
			Currency:       bucket.Currency,
			Maturity:       string(bucket.Maturity),
			SourceHash:     bucket.SourceHash,
			IngestionRunID: runID,
		}
		if err := model.UpsertUpstreamCostBucket(&record); err != nil {
			return counters, err
		}
		counters.Upserted++
	}
	return counters, nil
}

func BuildReconcileDailySummaries(
	configID int64,
	accountID string,
	regions []string,
	period reconcile.Period,
) ([]model.ReconcileDailySummary, error) {
	if configID == 0 {
		return nil, errors.New("reconciliation config id is required")
	}
	if err := period.Validate(); err != nil {
		return nil, err
	}
	invocationRecords, err := model.FindUpstreamInvocationsForReconcile(
		accountID,
		regions,
		period.Start.Unix(),
		period.End.Unix(),
	)
	if err != nil {
		return nil, err
	}
	costRecords, err := model.FindCostBucketsForReconcile(
		accountID,
		regions,
		period.Start.Unix(),
		period.End.Unix(),
	)
	if err != nil {
		return nil, err
	}

	invocations := make([]reconcile.Invocation, 0, len(invocationRecords))
	for _, record := range invocationRecords {
		invocations = append(invocations, reconcile.Invocation{
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
	costs := make([]reconcile.CostBucket, 0, len(costRecords))
	for _, record := range costRecords {
		costs = append(costs, reconcile.CostBucket{
			Provider:      record.Provider,
			AccountID:     record.AccountID,
			Period:        reconcile.Period{Start: time.Unix(record.PeriodStart, 0), End: time.Unix(record.PeriodEnd, 0)},
			Region:        record.Region,
			ModelID:       record.ModelID,
			Operation:     record.Operation,
			UsageType:     record.UsageType,
			TokenCategory: record.TokenCategory,
			ServiceTier:   record.ServiceTier,
			RoutingType:   record.RoutingType,
			UsageQuantity: record.UsageQuantity,
			UnblendedCost: record.UnblendedCost,
			NetCost:       record.NetCost,
			Currency:      record.Currency,
			SourceKey:     record.SourceKey,
			SourceHash:    record.SourceHash,
			Maturity:      reconcile.Maturity(record.Maturity),
		})
	}

	daily := reconcile.BuildDailySummaries(invocations, costs)
	result := make([]model.ReconcileDailySummary, 0, len(daily))
	for _, summary := range daily {
		dimension := summary.Dimension
		keySource := fmt.Sprintf("%d|%d|%s|%s|%d|%s|%s|%s|%s|%s",
			configID,
			dimension.Day,
			dimension.AccountID,
			dimension.Region,
			dimension.ChannelID,
			dimension.ModelID,
			dimension.Operation,
			dimension.ServiceTier,
			dimension.RoutingType,
			dimension.TokenCategory,
		)
		record := model.ReconcileDailySummary{
			SummaryKey:       fmt.Sprintf("%x", common.Sha256Raw([]byte(keySource))),
			ConfigID:         configID,
			Day:              dimension.Day,
			AccountID:        dimension.AccountID,
			Region:           dimension.Region,
			ChannelID:        dimension.ChannelID,
			ModelID:          dimension.ModelID,
			Operation:        dimension.Operation,
			ServiceTier:      dimension.ServiceTier,
			RoutingType:      dimension.RoutingType,
			TokenCategory:    dimension.TokenCategory,
			UpstreamRequests: summary.UpstreamRequests,
			UpstreamTokens:   summary.UpstreamTokens,
			CURCost:          summary.CURCost,
			Maturity:         string(summary.Maturity),
		}
		record.AbsoluteDelta = record.CURCost.Abs()
		if !record.CURCost.IsZero() {
			record.PercentageDelta = decimal.NewFromInt(100)
		}
		if err := model.UpsertReconcileDailySummary(&record); err != nil {
			return result, err
		}
		result = append(result, record)
	}
	return result, nil
}

func PersistReconcileAccountSummary(
	configID int64,
	accountID string,
	period reconcile.Period,
	costs []reconcile.CostBucket,
	adjustments []reconcile.AccountAdjustment,
	daily []reconcile.DailySummary,
) (model.ReconcileAccountSummary, error) {
	if configID <= 0 || accountID == "" {
		return model.ReconcileAccountSummary{}, errors.New("reconciliation config id and account id are required")
	}
	if err := period.Validate(); err != nil {
		return model.ReconcileAccountSummary{}, err
	}
	summary := reconcile.BuildAccountSummary(costs, adjustments, daily)
	keySource := fmt.Sprintf("%d|%s|%d|%d", configID, accountID, period.Start.Unix(), period.End.Unix())
	record := model.ReconcileAccountSummary{
		SummaryKey:        fmt.Sprintf("%x", common.Sha256Raw([]byte(keySource))),
		ConfigID:          configID,
		PeriodStart:       period.Start.Unix(),
		PeriodEnd:         period.End.Unix(),
		AccountID:         accountID,
		GrossCost:         summary.GrossCost,
		Credits:           summary.Credits,
		Refunds:           summary.Refunds,
		TaxAndAdjustments: summary.TaxAndAdjustments,
		NetCost:           summary.NetCost,
		AttributedCost:    summary.AttributedCost,
		UnattributedCost:  summary.UnattributedCost,
		UnexplainedDelta:  summary.UnexplainedDelta,
		Currency:          summary.Currency,
		Maturity:          string(summary.Maturity),
	}
	if err := model.UpsertReconcileAccountSummary(&record); err != nil {
		return model.ReconcileAccountSummary{}, err
	}
	return record, nil
}

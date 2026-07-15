package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
	bedrockreconcile "github.com/QuantumNous/new-api/reconcile/bedrock"
)

type BedrockReconcileRunResult struct {
	Invocation InvocationIngestionCounters                     `json:"invocation"`
	Costs      CostIngestionCounters                           `json:"costs"`
	Requests   ReconcileRequestCounters                        `json:"requests"`
	Daily      int                                             `json:"daily_summaries"`
	Account    *model.ReconcileAccountSummary                  `json:"account_summary,omitempty"`
	Metrics    map[string]bedrockreconcile.CompletenessSummary `json:"completeness"`
}

type BedrockReconcileTaskPayload struct {
	ConfigID    int64  `json:"config_id,omitempty"`
	PeriodStart int64  `json:"period_start,omitempty"`
	PeriodEnd   int64  `json:"period_end,omitempty"`
	ResumeRunID string `json:"resume_run_id,omitempty"`
}

func RunBedrockReconcileTask(ctx context.Context, payload BedrockReconcileTaskPayload, taskID string) (map[int64]BedrockReconcileRunResult, error) {
	configs, err := model.ListReconcileConfigs()
	if err != nil {
		return nil, err
	}
	result := make(map[int64]BedrockReconcileRunResult)
	matched := false
	for index := range configs {
		config := &configs[index]
		if payload.ConfigID > 0 && config.ID != payload.ConfigID {
			continue
		}
		if payload.ConfigID == 0 && !config.Enabled {
			continue
		}
		matched = true
		period := reconcile.Period{Start: time.Unix(payload.PeriodStart, 0), End: time.Unix(payload.PeriodEnd, 0)}
		if payload.PeriodStart == 0 || payload.PeriodEnd == 0 {
			period = DefaultReconcilePeriod(config, time.Now())
		}
		runID := fmt.Sprintf("%s-%d", taskID, config.ID)
		resumeCursor := ""
		if payload.ResumeRunID != "" {
			previousRun, previousErr := model.GetReconcileRun(payload.ResumeRunID)
			if previousErr != nil {
				return result, previousErr
			}
			if previousRun == nil || previousRun.ConfigID != config.ID {
				return result, errors.New("reconciliation resume run not found")
			}
			resumeCursor = previousRun.Cursor
		}
		runResult, runErr := runBedrockReconciliation(ctx, config.ID, period, runID, resumeCursor)
		result[config.ID] = runResult
		if runErr != nil {
			return result, fmt.Errorf("%s: %w", ReconcileRunLabel(config.ID, period), runErr)
		}
	}
	if !matched && payload.ConfigID > 0 {
		return result, errors.New("reconciliation config not found")
	}
	return result, nil
}

func NewBedrockReconcileProvider(ctx context.Context, config *model.ReconcileConfig, region string, period reconcile.Period, runID string) (*bedrockreconcile.AWSProvider, error) {
	if config == nil {
		return nil, errors.New("reconciliation config is required")
	}
	return bedrockreconcile.NewProvider(ctx, bedrockreconcile.ProviderConfig{
		Role: bedrockreconcile.RoleConfig{
			RoleARN: config.RoleARN, ExternalID: config.ExternalID, Region: region,
			SessionName: "lighttrust-" + common.NodeName + "-" + runID,
		},
		AccountID: config.AccountID, InvocationSource: config.InvocationSource,
		InvocationLogGroup: config.InvocationLogGroup, InvocationS3Bucket: config.InvocationS3Bucket,
		InvocationS3Prefix: config.InvocationS3Prefix, CostExplorerEnabled: config.CostExplorerEnabled, Period: period,
		Athena: bedrockreconcile.AthenaCURConfig{
			Database: config.AthenaDatabase, Table: config.AthenaTable, Workgroup: config.AthenaWorkgroup,
			OutputLocation: config.AthenaOutputLocation, AccountID: config.AccountID, Region: region,
			Maturity: reconcile.MaturityProvisional,
		},
	})
}

func RunBedrockReconciliation(ctx context.Context, configID int64, period reconcile.Period, runID string) (result BedrockReconcileRunResult, runErr error) {
	return runBedrockReconciliation(ctx, configID, period, runID, "")
}

func runBedrockReconciliation(ctx context.Context, configID int64, period reconcile.Period, runID string, resumeCursor string) (result BedrockReconcileRunResult, runErr error) {
	if err := period.Validate(); err != nil {
		return result, err
	}
	config, err := model.GetReconcileConfig(configID)
	if err != nil {
		return result, err
	}
	if config == nil {
		return result, errors.New("reconciliation config not found")
	}
	var regions []string
	if err := common.UnmarshalJsonStr(config.Regions, &regions); err != nil {
		return result, err
	}
	var mappings map[string][]int
	if err := common.UnmarshalJsonStr(config.ChannelMappings, &mappings); err != nil {
		return result, err
	}
	record := &model.ReconcileRun{
		RunID: runID, ConfigID: configID, Source: "bedrock_pipeline", Status: string(reconcile.RunStatusRunning),
		Maturity: string(reconcile.MaturityProvisional), PeriodStart: period.Start.Unix(), PeriodEnd: period.End.Unix(),
		Cursor: resumeCursor, LockedBy: common.NodeName,
	}
	if err := model.CreateReconcileRun(record); err != nil {
		return result, err
	}
	defer func() {
		status := reconcile.RunStatusSucceeded
		if runErr != nil {
			status = reconcile.RunStatusFailed
		}
		_ = model.FinishReconcileRun(runID, status, reconcile.MaturityProvisional, result, runErr)
	}()

	allCosts := make([]reconcile.CostBucket, 0)
	allDaily := make([]reconcile.DailySummary, 0)
	var adjustmentProvider *bedrockreconcile.AWSProvider
	result.Metrics = make(map[string]bedrockreconcile.CompletenessSummary, len(regions))
	channelIDs := make([]int, 0)
	cursors := make(map[string]reconcile.Cursor)
	if resumeCursor != "" {
		if err := common.UnmarshalJsonStr(resumeCursor, &cursors); err != nil {
			return result, fmt.Errorf("decode reconciliation resume cursor: %w", err)
		}
	}
	for _, region := range regions {
		channelIDs = append(channelIDs, mappings[region]...)
		provider, providerErr := NewBedrockReconcileProvider(ctx, config, region, period, runID)
		if providerErr != nil {
			return result, providerErr
		}
		if adjustmentProvider == nil {
			adjustmentProvider = provider
		}
		cursor := cursors[region]
		for page := 0; page < 1000; page++ {
			counters, nextCursor, ingestErr := IngestReconcileInvocations(ctx, provider, cursor, runID)
			if ingestErr != nil {
				return result, ingestErr
			}
			result.Invocation.Scanned += counters.Scanned
			result.Invocation.Upserted += counters.Upserted
			cursor = nextCursor
			cursors[region] = cursor
			encodedCursor, encodeErr := common.Marshal(cursors)
			if encodeErr != nil {
				return result, encodeErr
			}
			if persistErr := model.UpdateReconcileRunCursor(runID, string(encodedCursor)); persistErr != nil {
				return result, persistErr
			}
			if bedrockreconcile.InvocationCursorComplete(cursor) {
				break
			}
			if counters.Scanned == 0 {
				select {
				case <-ctx.Done():
					return result, ctx.Err()
				case <-time.After(time.Second):
				}
			}
			if page == 999 {
				return result, errors.New("Bedrock invocation ingestion exceeded page limit")
			}
		}
		costs, costErr := provider.PullDailyCosts(ctx, period)
		if costErr != nil {
			return result, costErr
		}
		costCounters, costErr := PersistReconcileCosts(costs, runID)
		if costErr != nil {
			return result, costErr
		}
		result.Costs.Scanned += costCounters.Scanned
		result.Costs.Upserted += costCounters.Upserted
		allCosts = append(allCosts, costs...)
		metrics, metricErr := provider.PullCompleteness(ctx, period)
		if metricErr != nil {
			return result, metricErr
		}
		result.Metrics[region] = metrics
	}

	requestCounters, err := ReconcileRequests(ReconcileRequestsInput{
		ConfigID: config.ID, AccountID: config.AccountID, Regions: regions, ChannelIDs: uniqueInts(channelIDs),
		Period: period, MaturityDelay: time.Duration(config.MaturityDelaySeconds) * time.Second, Now: time.Now(),
	})
	if err != nil {
		return result, err
	}
	result.Requests = requestCounters
	dailyRecords, err := BuildReconcileDailySummaries(config.ID, config.AccountID, regions, period)
	if err != nil {
		return result, err
	}
	result.Daily = len(dailyRecords)
	for _, record := range dailyRecords {
		allDaily = append(allDaily, reconcile.DailySummary{
			Dimension:        reconcile.DailyDimension{Day: record.Day, AccountID: record.AccountID, Region: record.Region, ChannelID: record.ChannelID, ModelID: record.ModelID, Operation: record.Operation, ServiceTier: record.ServiceTier, RoutingType: record.RoutingType, TokenCategory: record.TokenCategory, Currency: "USD"},
			UpstreamRequests: record.UpstreamRequests, UpstreamTokens: record.UpstreamTokens, CURCost: record.CURCost, Maturity: reconcile.Maturity(record.Maturity),
		})
	}
	adjustments, err := adjustmentProvider.PullAccountAdjustments(ctx, period)
	if err != nil {
		return result, err
	}
	account, err := PersistReconcileAccountSummary(config.ID, config.AccountID, period, allCosts, adjustments, allDaily)
	if err != nil {
		return result, err
	}
	result.Account = &account
	return result, nil
}

func uniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func DefaultReconcilePeriod(config *model.ReconcileConfig, now time.Time) reconcile.Period {
	days := config.LookbackDays
	if days <= 0 {
		days = 3
	}
	end := now.UTC().Truncate(24 * time.Hour)
	return reconcile.Period{Start: end.Add(-time.Duration(days) * 24 * time.Hour), End: end}
}

func ReconcileRunLabel(configID int64, period reconcile.Period) string {
	return fmt.Sprintf("config=%d period=%s/%s", configID, period.Start.UTC().Format(time.RFC3339), period.End.UTC().Format(time.RFC3339))
}

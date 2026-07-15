package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCostProvider struct {
	buckets []reconcile.CostBucket
	err     error
}

func (s stubCostProvider) PullDailyCosts(_ context.Context, _ reconcile.Period) ([]reconcile.CostBucket, error) {
	return s.buckets, s.err
}

func TestIngestCostsAndBuildDailySummariesIdempotently(t *testing.T) {
	require.NoError(t, model.DB.Exec("DELETE FROM upstream_invocations").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM upstream_cost_buckets").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM reconcile_daily_summaries").Error)
	t.Cleanup(func() {
		require.NoError(t, model.DB.Exec("DELETE FROM upstream_invocations").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM upstream_cost_buckets").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM reconcile_daily_summaries").Error)
	})

	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	period := reconcile.Period{Start: day, End: day.Add(24 * time.Hour)}
	provider := stubCostProvider{buckets: []reconcile.CostBucket{{
		Provider:      reconcile.ProviderBedrock,
		AccountID:     "123456789012",
		Period:        period,
		Region:        "us-east-1",
		ModelID:       "model",
		Operation:     "InvokeModel",
		TokenCategory: "input",
		ServiceTier:   "standard",
		RoutingType:   "in_region",
		UsageQuantity: decimal.NewFromInt(100),
		UnblendedCost: decimal.NewFromInt(1),
		NetCost:       decimal.NewFromInt(1),
		Currency:      "USD",
		SourceKey:     "cost-1",
		SourceHash:    "hash-1",
		Maturity:      reconcile.MaturityFinal,
	}}}

	for range 2 {
		counters, err := IngestReconcileCosts(context.Background(), provider, period, "run-1")
		require.NoError(t, err)
		assert.Equal(t, CostIngestionCounters{Scanned: 1, Upserted: 1}, counters)
	}
	var costCount int64
	require.NoError(t, model.DB.Model(&model.UpstreamCostBucket{}).Count(&costCount).Error)
	assert.Equal(t, int64(1), costCount)

	require.NoError(t, model.DB.Create(&model.UpstreamInvocation{
		Provider:          reconcile.ProviderBedrock,
		AccountID:         "123456789012",
		Region:            "us-east-1",
		RequestID:         "request-1",
		ChannelID:         42,
		InvokedAt:         day.Add(time.Hour).Unix(),
		Operation:         "InvokeModel",
		ModelID:           "model",
		NormalizedModelID: "model",
		ServiceTier:       "standard",
		RoutingType:       "in_region",
		InputTokens:       100,
	}).Error)

	for range 2 {
		summaries, err := BuildReconcileDailySummaries(1, "123456789012", []string{"us-east-1"}, period)
		require.NoError(t, err)
		require.Len(t, summaries, 1)
		assert.Equal(t, 42, summaries[0].ChannelID)
		assert.True(t, summaries[0].CURCost.Equal(decimal.NewFromInt(1)))
		assert.Equal(t, string(reconcile.MaturityFinal), summaries[0].Maturity)
	}
	var summaryCount int64
	require.NoError(t, model.DB.Model(&model.ReconcileDailySummary{}).Count(&summaryCount).Error)
	assert.Equal(t, int64(1), summaryCount)
}

package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileConfigValidateAppliesSafeDefaults(t *testing.T) {
	config := &ReconcileConfig{
		Name:            " Bedrock production ",
		Provider:        " bedrock ",
		AccountID:       " 123456789012 ",
		RoleARN:         " arn:aws:iam::123456789012:role/Reconcile ",
		ExternalID:      " external ",
		Regions:         `["us-east-1"]`,
		ChannelMappings: `{"us-east-1":[1]}`,
	}

	require.NoError(t, config.Validate())
	assert.Equal(t, "Bedrock production", config.Name)
	assert.Equal(t, "bedrock", config.Provider)
	assert.Equal(t, int64(1800), config.MaturityDelaySeconds)
	assert.Equal(t, 3, config.LookbackDays)
	assert.Equal(t, "0", config.Tolerance)
}

func TestUpsertUpstreamInvocationIsIdempotent(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM upstream_invocations").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Exec("DELETE FROM upstream_invocations").Error)
	})

	record := &UpstreamInvocation{
		Provider:     "bedrock",
		AccountID:    "123456789012",
		Region:       "us-east-1",
		RequestID:    "aws-request-1",
		InputTokens:  10,
		OutputTokens: 20,
		SourceHash:   "old",
	}
	require.NoError(t, UpsertUpstreamInvocation(record))

	updated := *record
	updated.ID = 0
	updated.InputTokens = 11
	updated.SourceHash = "new"
	require.NoError(t, UpsertUpstreamInvocation(&updated))

	var count int64
	require.NoError(t, DB.Model(&UpstreamInvocation{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	var stored UpstreamInvocation
	require.NoError(t, DB.First(&stored).Error)
	assert.Equal(t, int64(11), stored.InputTokens)
	assert.Equal(t, "new", stored.SourceHash)
}

func TestUpsertUpstreamCostBucketPreservesDecimalPrecision(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM upstream_cost_buckets").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Exec("DELETE FROM upstream_cost_buckets").Error)
	})

	record := &UpstreamCostBucket{
		SourceKey:     "bucket-1",
		Provider:      "bedrock",
		AccountID:     "123456789012",
		UsageQuantity: decimal.RequireFromString("123456789.123456789012345678"),
		UnblendedCost: decimal.RequireFromString("0.000000000123456789"),
		NetCost:       decimal.RequireFromString("0.000000000123456789"),
		Currency:      "USD",
	}
	require.NoError(t, UpsertUpstreamCostBucket(record))

	updated := *record
	updated.ID = 0
	updated.NetCost = decimal.RequireFromString("0.000000000987654321")
	require.NoError(t, UpsertUpstreamCostBucket(&updated))

	var stored UpstreamCostBucket
	require.NoError(t, DB.Where("source_key = ?", "bucket-1").First(&stored).Error)
	assert.True(t, stored.UsageQuantity.Equal(record.UsageQuantity))
	assert.True(t, stored.NetCost.Equal(updated.NetCost))
}

func TestListReconcileItemsFiltersByChannelAndPaginates(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM reconcile_items").Error)
	require.NoError(t, DB.Exec("DELETE FROM upstream_invocations").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Exec("DELETE FROM reconcile_items").Error)
		require.NoError(t, DB.Exec("DELETE FROM upstream_invocations").Error)
	})

	invocation := &UpstreamInvocation{Provider: "bedrock", AccountID: "123456789012", Region: "us-east-1", RequestID: "aws-1", ChannelID: 7}
	require.NoError(t, DB.Create(invocation).Error)
	require.NoError(t, DB.Create(&ReconcileItem{ItemKey: "item-1", ConfigID: 11, UpstreamInvocationID: invocation.ID, Status: "matched", LastObservedAt: 10}).Error)
	require.NoError(t, DB.Create(&ReconcileItem{ItemKey: "item-2", ConfigID: 11, Status: "missing_upstream", LastObservedAt: 20}).Error)

	items, total, err := ListReconcileItems(ReconcileResultFilter{ConfigID: 11, ChannelID: 7, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "item-1", items[0].ItemKey)
}

func TestUpsertAndListReconcileAccountSummary(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM reconcile_account_summaries").Error)
	t.Cleanup(func() { require.NoError(t, DB.Exec("DELETE FROM reconcile_account_summaries").Error) })

	summary := &ReconcileAccountSummary{
		SummaryKey: "account-1", ConfigID: 3, PeriodStart: 100, PeriodEnd: 200,
		AccountID: "123456789012", NetCost: decimal.RequireFromString("1.25"), Currency: "USD", Maturity: "final",
	}
	require.NoError(t, UpsertReconcileAccountSummary(summary))
	summary.ID = 0
	summary.NetCost = decimal.RequireFromString("1.50")
	require.NoError(t, UpsertReconcileAccountSummary(summary))

	items, total, err := ListReconcileAccountSummaries(ReconcileResultFilter{ConfigID: 3, Start: 150, End: 250, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.True(t, items[0].NetCost.Equal(decimal.RequireFromString("1.50")))
}

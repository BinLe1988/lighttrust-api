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

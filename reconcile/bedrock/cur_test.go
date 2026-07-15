package bedrock

import (
	"testing"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCURRowPreservesBillingDimensionsAndPrecision(t *testing.T) {
	row := CURRow{
		AccountID:     "123456789012",
		PeriodStart:   "2026-07-15T00:00:00Z",
		PeriodEnd:     "2026-07-16T00:00:00Z",
		Region:        "us-east-1",
		ModelID:       "us.anthropic.claude-sonnet-4-20250514-v1:0",
		Operation:     "InvokeModel",
		UsageType:     "USE1-anthropic.claude-cache-read-input-token-count-cross-region-global-priority",
		UsageQuantity: "123456789.123456789012345678",
		UnblendedCost: "0.000000000123456789",
		NetCost:       "0.000000000120000001",
		Currency:      "usd",
		LineItemID:    "line-1",
	}

	bucket, err := NormalizeCURRow(row, reconcile.MaturityFinal)
	require.NoError(t, err)
	assert.Equal(t, TokenCategoryCacheRead, bucket.TokenCategory)
	assert.Equal(t, "priority", bucket.ServiceTier)
	assert.Equal(t, "cross_region_global", bucket.RoutingType)
	assert.Equal(t, "anthropic.claude-sonnet-4-20250514-v1:0", bucket.ModelID)
	assert.True(t, bucket.UsageQuantity.Equal(decimal.RequireFromString(row.UsageQuantity)))
	assert.True(t, bucket.NetCost.Equal(decimal.RequireFromString(row.NetCost)))
	assert.Equal(t, "USD", bucket.Currency)
	assert.NotEmpty(t, bucket.SourceKey)
	assert.NotEmpty(t, bucket.SourceHash)
}

func TestNormalizeCURRowRejectsNegativeUsage(t *testing.T) {
	_, err := NormalizeCURRow(CURRow{
		AccountID:     "1",
		PeriodStart:   "2026-07-15T00:00:00Z",
		PeriodEnd:     "2026-07-16T00:00:00Z",
		UsageType:     "input-tokens",
		UsageQuantity: "-1",
		UnblendedCost: "0",
		Currency:      "USD",
	}, reconcile.MaturityFinal)
	require.ErrorContains(t, err, "cannot be negative")
}

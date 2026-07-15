package reconcile

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDailySummariesAllocatesCURCostByChannelUsage(t *testing.T) {
	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	invocations := []Invocation{
		{AccountID: "1", Region: "us-east-1", ChannelID: 10, Timestamp: day.Add(time.Hour), ModelID: "model", Operation: "InvokeModel", InputTokens: 25, ServiceTier: "standard", RoutingType: "in_region"},
		{AccountID: "1", Region: "us-east-1", ChannelID: 20, Timestamp: day.Add(2 * time.Hour), ModelID: "model", Operation: "InvokeModel", InputTokens: 75, ServiceTier: "standard", RoutingType: "in_region"},
	}
	costs := []CostBucket{{
		Provider:      ProviderBedrock,
		AccountID:     "1",
		Period:        Period{Start: day, End: day.Add(24 * time.Hour)},
		Region:        "us-east-1",
		ModelID:       "model",
		Operation:     "InvokeModel",
		TokenCategory: "input",
		ServiceTier:   "standard",
		RoutingType:   "in_region",
		UsageQuantity: decimal.NewFromInt(100),
		NetCost:       decimal.NewFromInt(1),
		Currency:      "USD",
		Maturity:      MaturityFinal,
	}}

	summaries := BuildDailySummaries(invocations, costs)
	require.Len(t, summaries, 2)
	assert.Equal(t, 10, summaries[0].Dimension.ChannelID)
	assert.True(t, summaries[0].CURCost.Equal(decimal.RequireFromString("0.25")))
	assert.Equal(t, 20, summaries[1].Dimension.ChannelID)
	assert.True(t, summaries[1].CURCost.Equal(decimal.RequireFromString("0.75")))
	assert.Equal(t, MaturityFinal, summaries[0].Maturity)
}

func TestBuildDailySummariesKeepsUnattributedCURCost(t *testing.T) {
	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	costs := []CostBucket{{
		AccountID:     "1",
		Period:        Period{Start: day, End: day.Add(time.Hour)},
		Region:        "us-east-1",
		ModelID:       "model",
		Operation:     "InvokeModel",
		TokenCategory: "input",
		UsageQuantity: decimal.NewFromInt(10),
		NetCost:       decimal.RequireFromString("0.5"),
		Currency:      "USD",
		Maturity:      MaturityProvisional,
	}}

	summaries := BuildDailySummaries(nil, costs)
	require.Len(t, summaries, 1)
	assert.Equal(t, 0, summaries[0].Dimension.ChannelID)
	assert.True(t, summaries[0].CURCost.Equal(decimal.RequireFromString("0.5")))
	assert.Equal(t, MaturityProvisional, summaries[0].Maturity)
}

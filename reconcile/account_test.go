package reconcile

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestBuildAccountSummarySatisfiesCostIdentity(t *testing.T) {
	costs := []CostBucket{
		{UnblendedCost: decimal.NewFromInt(10), Currency: "USD", Maturity: MaturityFinal},
		{UnblendedCost: decimal.NewFromInt(5), Currency: "USD", Maturity: MaturityFinal},
	}
	adjustments := []AccountAdjustment{
		{Type: AdjustmentCredit, Amount: decimal.NewFromInt(2), Currency: "USD"},
		{Type: AdjustmentRefund, Amount: decimal.NewFromInt(1), Currency: "USD"},
		{Type: AdjustmentTax, Amount: decimal.RequireFromString("0.5"), Currency: "USD"},
	}
	daily := []DailySummary{
		{Dimension: DailyDimension{ChannelID: 42}, CURCost: decimal.NewFromInt(10), Maturity: MaturityFinal},
		{Dimension: DailyDimension{ChannelID: 0}, CURCost: decimal.NewFromInt(2), Maturity: MaturityFinal},
	}

	summary := BuildAccountSummary(costs, adjustments, daily)
	assert.True(t, summary.GrossCost.Equal(decimal.NewFromInt(15)))
	assert.True(t, summary.NetCost.Equal(decimal.RequireFromString("12.5")))
	assert.True(t, summary.AttributedCost.Equal(decimal.NewFromInt(10)))
	assert.True(t, summary.UnattributedCost.Equal(decimal.NewFromInt(2)))
	assert.True(t, summary.UnexplainedDelta.Equal(decimal.RequireFromString("0.5")))
	assert.Equal(t, MaturityFinal, summary.Maturity)
}

func TestBuildAccountSummaryRemainsPendingWithoutCUR(t *testing.T) {
	summary := BuildAccountSummary(nil, nil, nil)
	assert.Equal(t, MaturityPending, summary.Maturity)
}

package reconcile

import "github.com/shopspring/decimal"

type AccountSummary struct {
	GrossCost         decimal.Decimal
	Credits           decimal.Decimal
	Refunds           decimal.Decimal
	TaxAndAdjustments decimal.Decimal
	NetCost           decimal.Decimal
	AttributedCost    decimal.Decimal
	UnattributedCost  decimal.Decimal
	UnexplainedDelta  decimal.Decimal
	Currency          string
	Maturity          Maturity
}

func BuildAccountSummary(
	costs []CostBucket,
	adjustments []AccountAdjustment,
	daily []DailySummary,
) AccountSummary {
	summary := AccountSummary{Maturity: MaturityFinal}
	for _, cost := range costs {
		summary.GrossCost = summary.GrossCost.Add(cost.UnblendedCost)
		if summary.Currency == "" {
			summary.Currency = cost.Currency
		}
		summary.Maturity = leastMature(summary.Maturity, cost.Maturity)
	}
	for _, adjustment := range adjustments {
		amount := adjustment.Amount.Abs()
		switch adjustment.Type {
		case AdjustmentCredit:
			summary.Credits = summary.Credits.Add(amount)
		case AdjustmentRefund:
			summary.Refunds = summary.Refunds.Add(amount)
		case AdjustmentTax:
			summary.TaxAndAdjustments = summary.TaxAndAdjustments.Add(amount)
		default:
			summary.TaxAndAdjustments = summary.TaxAndAdjustments.Add(adjustment.Amount)
		}
		if summary.Currency == "" {
			summary.Currency = adjustment.Currency
		}
	}
	for _, dailySummary := range daily {
		if dailySummary.Dimension.ChannelID == 0 {
			summary.UnattributedCost = summary.UnattributedCost.Add(dailySummary.CURCost)
		} else {
			summary.AttributedCost = summary.AttributedCost.Add(dailySummary.CURCost)
		}
		summary.Maturity = leastMature(summary.Maturity, dailySummary.Maturity)
	}
	summary.NetCost = summary.GrossCost.
		Sub(summary.Credits).
		Sub(summary.Refunds).
		Add(summary.TaxAndAdjustments)
	summary.UnexplainedDelta = summary.NetCost.
		Sub(summary.AttributedCost).
		Sub(summary.UnattributedCost)
	if len(costs) == 0 {
		summary.Maturity = MaturityPending
	}
	return summary
}

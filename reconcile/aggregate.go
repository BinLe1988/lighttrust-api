package reconcile

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type DailyDimension struct {
	Day           int64
	AccountID     string
	Region        string
	ChannelID     int
	ModelID       string
	Operation     string
	ServiceTier   string
	RoutingType   string
	TokenCategory string
	Currency      string
}

type DailySummary struct {
	Dimension        DailyDimension
	UpstreamRequests int64
	UpstreamTokens   int64
	CURUsage         decimal.Decimal
	CURCost          decimal.Decimal
	Maturity         Maturity
}

type dailyAllocationKey struct {
	Day           int64
	AccountID     string
	Region        string
	ModelID       string
	Operation     string
	ServiceTier   string
	RoutingType   string
	TokenCategory string
}

func BuildDailySummaries(invocations []Invocation, costs []CostBucket) []DailySummary {
	summaries := make(map[DailyDimension]*DailySummary)
	allocationDimensions := make(map[dailyAllocationKey][]DailyDimension)
	costApplied := make(map[DailyDimension]bool)

	for _, invocation := range invocations {
		day := startOfUTCDay(invocation.Timestamp).Unix()
		categories := []struct {
			name   string
			tokens int64
		}{
			{name: "input", tokens: invocation.InputTokens},
			{name: "output", tokens: invocation.OutputTokens},
			{name: "cache_read_input", tokens: invocation.CacheReadInputTokens},
			{name: "cache_write_input", tokens: invocation.CacheWriteInputTokens},
		}
		for _, category := range categories {
			if category.tokens == 0 {
				continue
			}
			dimension := DailyDimension{
				Day:           day,
				AccountID:     invocation.AccountID,
				Region:        invocation.Region,
				ChannelID:     invocation.ChannelID,
				ModelID:       normalizeComparableModel(invocation.NormalizedModelID, invocation.ModelID),
				Operation:     invocation.Operation,
				ServiceTier:   defaultString(invocation.ServiceTier, "standard"),
				RoutingType:   defaultString(invocation.RoutingType, "in_region"),
				TokenCategory: category.name,
				Currency:      "USD",
			}
			summary := summaries[dimension]
			if summary == nil {
				summary = &DailySummary{Dimension: dimension, Maturity: MaturityFinal}
				summaries[dimension] = summary
				allocationKey := allocationKeyForDimension(dimension)
				allocationDimensions[allocationKey] = append(allocationDimensions[allocationKey], dimension)
			}
			summary.UpstreamRequests++
			summary.UpstreamTokens += category.tokens
		}
	}

	for _, cost := range costs {
		key := dailyAllocationKey{
			Day:           startOfUTCDay(cost.Period.Start).Unix(),
			AccountID:     cost.AccountID,
			Region:        cost.Region,
			ModelID:       normalizeComparableModel(cost.ModelID, cost.ModelID),
			Operation:     cost.Operation,
			ServiceTier:   defaultString(cost.ServiceTier, "standard"),
			RoutingType:   defaultString(cost.RoutingType, "in_region"),
			TokenCategory: cost.TokenCategory,
		}
		dimensions := allocationDimensions[key]
		if len(dimensions) == 0 {
			dimension := DailyDimension{
				Day:           key.Day,
				AccountID:     key.AccountID,
				Region:        key.Region,
				ModelID:       key.ModelID,
				Operation:     key.Operation,
				ServiceTier:   key.ServiceTier,
				RoutingType:   key.RoutingType,
				TokenCategory: key.TokenCategory,
				Currency:      cost.Currency,
			}
			summary := summaries[dimension]
			if summary == nil {
				summary = &DailySummary{Dimension: dimension, Maturity: cost.Maturity}
				summaries[dimension] = summary
			}
			summary.CURUsage = summary.CURUsage.Add(cost.UsageQuantity)
			summary.CURCost = summary.CURCost.Add(cost.NetCost)
			summary.Maturity = leastMature(summary.Maturity, cost.Maturity)
			costApplied[dimension] = true
			continue
		}

		totalTokens := int64(0)
		for _, dimension := range dimensions {
			totalTokens += summaries[dimension].UpstreamTokens
		}
		if totalTokens == 0 {
			continue
		}
		for _, dimension := range dimensions {
			summary := summaries[dimension]
			ratio := decimal.NewFromInt(summary.UpstreamTokens).Div(decimal.NewFromInt(totalTokens))
			summary.CURUsage = summary.CURUsage.Add(cost.UsageQuantity.Mul(ratio))
			summary.CURCost = summary.CURCost.Add(cost.NetCost.Mul(ratio))
			summary.Maturity = leastMature(summary.Maturity, cost.Maturity)
			costApplied[dimension] = true
		}
	}

	result := make([]DailySummary, 0, len(summaries))
	for dimension, summary := range summaries {
		if !costApplied[dimension] {
			summary.Maturity = MaturityPending
		}
		result = append(result, *summary)
	}
	sort.Slice(result, func(i, j int) bool {
		left := result[i].Dimension
		right := result[j].Dimension
		if left.Day != right.Day {
			return left.Day < right.Day
		}
		if left.ChannelID != right.ChannelID {
			return left.ChannelID < right.ChannelID
		}
		if left.ModelID != right.ModelID {
			return left.ModelID < right.ModelID
		}
		return left.TokenCategory < right.TokenCategory
	})
	return result
}

func allocationKeyForDimension(dimension DailyDimension) dailyAllocationKey {
	return dailyAllocationKey{
		Day:           dimension.Day,
		AccountID:     dimension.AccountID,
		Region:        dimension.Region,
		ModelID:       dimension.ModelID,
		Operation:     dimension.Operation,
		ServiceTier:   dimension.ServiceTier,
		RoutingType:   dimension.RoutingType,
		TokenCategory: dimension.TokenCategory,
	}
}

func startOfUTCDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func leastMature(left Maturity, right Maturity) Maturity {
	if left == MaturityPending || right == MaturityPending {
		return MaturityPending
	}
	if left == MaturityProvisional || right == MaturityProvisional {
		return MaturityProvisional
	}
	return MaturityFinal
}

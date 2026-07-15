package bedrock

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/shopspring/decimal"
)

const (
	TokenCategoryInput      = "input"
	TokenCategoryOutput     = "output"
	TokenCategoryCacheRead  = "cache_read_input"
	TokenCategoryCacheWrite = "cache_write_input"
	TokenCategoryOther      = "other"
)

type CURRow struct {
	AccountID     string
	PeriodStart   string
	PeriodEnd     string
	Region        string
	ModelID       string
	Operation     string
	UsageType     string
	UsageQuantity string
	UnblendedCost string
	NetCost       string
	Currency      string
	LineItemID    string
}

func NormalizeCURRow(row CURRow, maturity reconcile.Maturity) (reconcile.CostBucket, error) {
	periodStart, err := time.Parse(time.RFC3339, row.PeriodStart)
	if err != nil {
		return reconcile.CostBucket{}, fmt.Errorf("parse CUR period start: %w", err)
	}
	periodEnd, err := time.Parse(time.RFC3339, row.PeriodEnd)
	if err != nil {
		return reconcile.CostBucket{}, fmt.Errorf("parse CUR period end: %w", err)
	}
	usageQuantity, err := decimal.NewFromString(strings.TrimSpace(row.UsageQuantity))
	if err != nil {
		return reconcile.CostBucket{}, fmt.Errorf("parse CUR usage quantity: %w", err)
	}
	unblendedCost, err := decimal.NewFromString(strings.TrimSpace(row.UnblendedCost))
	if err != nil {
		return reconcile.CostBucket{}, fmt.Errorf("parse CUR unblended cost: %w", err)
	}
	netCost := unblendedCost
	if strings.TrimSpace(row.NetCost) != "" {
		netCost, err = decimal.NewFromString(strings.TrimSpace(row.NetCost))
		if err != nil {
			return reconcile.CostBucket{}, fmt.Errorf("parse CUR net cost: %w", err)
		}
	}

	usageType := strings.ToLower(strings.TrimSpace(row.UsageType))
	if usageType == "" {
		return reconcile.CostBucket{}, errors.New("CUR usage type is required")
	}
	sourceIdentity := strings.Join([]string{
		row.AccountID,
		row.PeriodStart,
		row.PeriodEnd,
		row.Region,
		row.ModelID,
		row.Operation,
		row.UsageType,
		row.LineItemID,
	}, "|")
	bucket := reconcile.CostBucket{
		Provider:      reconcile.ProviderBedrock,
		AccountID:     strings.TrimSpace(row.AccountID),
		Period:        reconcile.Period{Start: periodStart, End: periodEnd},
		Region:        strings.TrimSpace(row.Region),
		ModelID:       normalizeModelID(row.ModelID),
		Operation:     strings.TrimSpace(row.Operation),
		UsageType:     strings.TrimSpace(row.UsageType),
		TokenCategory: tokenCategoryFromUsageType(usageType),
		ServiceTier:   serviceTierFromUsageType(usageType),
		RoutingType:   routingTypeFromUsageType(usageType),
		UsageQuantity: usageQuantity,
		UnblendedCost: unblendedCost,
		NetCost:       netCost,
		Currency:      strings.ToUpper(strings.TrimSpace(row.Currency)),
		SourceKey:     fmt.Sprintf("%x", common.Sha256Raw([]byte(sourceIdentity))),
		Maturity:      maturity,
	}
	sourceBytes, err := common.Marshal(row)
	if err != nil {
		return reconcile.CostBucket{}, fmt.Errorf("hash CUR row: %w", err)
	}
	bucket.SourceHash = fmt.Sprintf("%x", common.Sha256Raw(sourceBytes))
	if err := bucket.Validate(); err != nil {
		return reconcile.CostBucket{}, err
	}
	return bucket, nil
}

func tokenCategoryFromUsageType(usageType string) string {
	switch {
	case strings.Contains(usageType, "cache-read-input-token"):
		return TokenCategoryCacheRead
	case strings.Contains(usageType, "cache-write-input-token"):
		return TokenCategoryCacheWrite
	case strings.Contains(usageType, "output-token"):
		return TokenCategoryOutput
	case strings.Contains(usageType, "input-token"):
		return TokenCategoryInput
	default:
		return TokenCategoryOther
	}
}

func serviceTierFromUsageType(usageType string) string {
	switch {
	case strings.Contains(usageType, "-priority"):
		return "priority"
	case strings.Contains(usageType, "-flex"):
		return "flex"
	case strings.Contains(usageType, "-reserved"):
		return "reserved"
	default:
		return "standard"
	}
}

func routingTypeFromUsageType(usageType string) string {
	switch {
	case strings.Contains(usageType, "cross-region-global"):
		return "cross_region_global"
	case strings.Contains(usageType, "cross-region"):
		return "cross_region"
	default:
		return "in_region"
	}
}

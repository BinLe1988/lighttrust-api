package bedrock

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/shopspring/decimal"
)

type costExplorerAPI interface {
	GetCostAndUsage(context.Context, *costexplorer.GetCostAndUsageInput, ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

type CostExplorerProvider struct {
	client    costExplorerAPI
	accountID string
}

func NewCostExplorerProvider(cfg aws.Config, accountID string) (*CostExplorerProvider, error) {
	return newCostExplorerProvider(costexplorer.NewFromConfig(cfg), accountID)
}

func newCostExplorerProvider(client costExplorerAPI, accountID string) (*CostExplorerProvider, error) {
	if client == nil || !validateAthenaAccountID(accountID) {
		return nil, errors.New("Cost Explorer client and 12-digit account id are required")
	}
	return &CostExplorerProvider{client: client, accountID: accountID}, nil
}

func (provider *CostExplorerProvider) PullAccountAdjustments(ctx context.Context, period reconcile.Period) ([]reconcile.AccountAdjustment, error) {
	if err := period.Validate(); err != nil {
		return nil, err
	}
	nextToken := ""
	adjustments := make([]reconcile.AccountAdjustment, 0)
	for {
		output, err := provider.client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
			TimePeriod:  &cetypes.DateInterval{Start: aws.String(period.Start.UTC().Format("2006-01-02")), End: aws.String(period.End.UTC().Format("2006-01-02"))},
			Granularity: cetypes.GranularityDaily,
			Metrics:     []string{"UnblendedCost", "NetUnblendedCost"},
			Filter: &cetypes.Expression{And: []cetypes.Expression{
				{Dimensions: &cetypes.DimensionValues{Key: cetypes.DimensionService, Values: []string{"Amazon Bedrock"}}},
				{Dimensions: &cetypes.DimensionValues{Key: cetypes.DimensionLinkedAccount, Values: []string{provider.accountID}}},
			}},
			GroupBy:       []cetypes.GroupDefinition{{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("RECORD_TYPE")}},
			NextPageToken: optionalString(nextToken),
		})
		if err != nil {
			return nil, safeAWSError("read Cost Explorer adjustments", err)
		}
		for _, result := range output.ResultsByTime {
			for _, group := range result.Groups {
				if len(group.Keys) == 0 {
					continue
				}
				adjustmentType, ok := normalizeAdjustmentType(group.Keys[0])
				if !ok {
					continue
				}
				metric, ok := group.Metrics["NetUnblendedCost"]
				if !ok || aws.ToString(metric.Amount) == "" {
					metric = group.Metrics["UnblendedCost"]
				}
				amount, parseErr := decimal.NewFromString(aws.ToString(metric.Amount))
				if parseErr != nil {
					return nil, fmt.Errorf("parse Cost Explorer adjustment amount: %w", parseErr)
				}
				start, end, parseErr := parseCostExplorerPeriod(result.TimePeriod)
				if parseErr != nil {
					return nil, parseErr
				}
				identity := strings.Join([]string{provider.accountID, start.Format("2006-01-02"), adjustmentType, group.Keys[0]}, "|")
				adjustments = append(adjustments, reconcile.AccountAdjustment{
					Provider: reconcile.ProviderBedrock, AccountID: provider.accountID,
					Period: reconcile.Period{Start: start, End: end}, Type: adjustmentType,
					Amount: amount, Currency: aws.ToString(metric.Unit), SourceKey: fmt.Sprintf("%x", common.Sha256Raw([]byte(identity))),
				})
			}
		}
		nextToken = aws.ToString(output.NextPageToken)
		if nextToken == "" {
			break
		}
	}
	return adjustments, nil
}

func normalizeAdjustmentType(recordType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(recordType)) {
	case "credit", "discount", "bundled discount":
		return reconcile.AdjustmentCredit, true
	case "refund":
		return reconcile.AdjustmentRefund, true
	case "tax":
		return reconcile.AdjustmentTax, true
	default:
		return "", false
	}
}

func parseCostExplorerPeriod(period *cetypes.DateInterval) (start, end time.Time, err error) {
	if period == nil {
		return start, end, errors.New("Cost Explorer result has no period")
	}
	start, err = time.Parse("2006-01-02", aws.ToString(period.Start))
	if err != nil {
		return start, end, fmt.Errorf("parse Cost Explorer period start: %w", err)
	}
	end, err = time.Parse("2006-01-02", aws.ToString(period.End))
	if err != nil {
		return start, end, fmt.Errorf("parse Cost Explorer period end: %w", err)
	}
	return start, end, nil
}

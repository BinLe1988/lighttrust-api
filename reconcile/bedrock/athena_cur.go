package bedrock

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
)

type athenaAPI interface {
	StartQueryExecution(context.Context, *athena.StartQueryExecutionInput, ...func(*athena.Options)) (*athena.StartQueryExecutionOutput, error)
	GetQueryExecution(context.Context, *athena.GetQueryExecutionInput, ...func(*athena.Options)) (*athena.GetQueryExecutionOutput, error)
	GetQueryResults(context.Context, *athena.GetQueryResultsInput, ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error)
}

type AthenaCURConfig struct {
	Database       string
	Table          string
	Workgroup      string
	OutputLocation string
	AccountID      string
	Region         string
	Maturity       reconcile.Maturity
}

type AthenaCURProvider struct {
	client athenaAPI
	config AthenaCURConfig
}

var athenaIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func NewAthenaCURProvider(cfg aws.Config, cur AthenaCURConfig) (*AthenaCURProvider, error) {
	return newAthenaCURProvider(athena.NewFromConfig(cfg), cur)
}

func newAthenaCURProvider(client athenaAPI, cur AthenaCURConfig) (*AthenaCURProvider, error) {
	if client == nil {
		return nil, errors.New("Athena client is required")
	}
	if !athenaIdentifier.MatchString(cur.Database) || !athenaIdentifier.MatchString(cur.Table) {
		return nil, errors.New("Athena database and table must be safe SQL identifiers")
	}
	if cur.Workgroup == "" || cur.OutputLocation == "" || cur.AccountID == "" {
		return nil, errors.New("Athena workgroup, output location, and account id are required")
	}
	if !validateAthenaAccountID(cur.AccountID) {
		return nil, errors.New("Athena account id must contain exactly 12 digits")
	}
	if !regexp.MustCompile(`^[a-z]{2}(?:-gov)?-[a-z]+-\d$`).MatchString(cur.Region) {
		return nil, errors.New("Athena CUR region is invalid")
	}
	if cur.Maturity == "" {
		cur.Maturity = reconcile.MaturityProvisional
	}
	return &AthenaCURProvider{client: client, config: cur}, nil
}

func (provider *AthenaCURProvider) PullDailyCosts(ctx context.Context, period reconcile.Period) ([]reconcile.CostBucket, error) {
	if err := period.Validate(); err != nil {
		return nil, err
	}
	query := provider.costQuery(period)
	token := fmt.Sprintf("%x", common.Sha256Raw([]byte(query)))
	started, err := provider.client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(query), ClientRequestToken: aws.String(token), WorkGroup: aws.String(provider.config.Workgroup),
		QueryExecutionContext: &athenatypes.QueryExecutionContext{Database: aws.String(provider.config.Database)},
		ResultConfiguration:   &athenatypes.ResultConfiguration{OutputLocation: aws.String(provider.config.OutputLocation)},
	})
	if err != nil {
		return nil, safeAWSError("start Athena CUR query", err)
	}
	queryID := aws.ToString(started.QueryExecutionId)
	if queryID == "" {
		return nil, errors.New("start Athena CUR query returned no execution id")
	}
	if err := provider.waitForQuery(ctx, queryID); err != nil {
		return nil, err
	}
	return provider.readCostResults(ctx, queryID)
}

func (provider *AthenaCURProvider) waitForQuery(ctx context.Context, queryID string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		output, err := provider.client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{QueryExecutionId: aws.String(queryID)})
		if err != nil {
			return safeAWSError("poll Athena CUR query", err)
		}
		if output.QueryExecution == nil || output.QueryExecution.Status == nil {
			return errors.New("Athena CUR query returned no status")
		}
		switch output.QueryExecution.Status.State {
		case athenatypes.QueryExecutionStateSucceeded:
			return nil
		case athenatypes.QueryExecutionStateFailed, athenatypes.QueryExecutionStateCancelled:
			return fmt.Errorf("Athena CUR query ended with status %s", output.QueryExecution.Status.State)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (provider *AthenaCURProvider) readCostResults(ctx context.Context, queryID string) ([]reconcile.CostBucket, error) {
	var result []reconcile.CostBucket
	nextToken := ""
	firstPage := true
	for {
		output, err := provider.client.GetQueryResults(ctx, &athena.GetQueryResultsInput{
			QueryExecutionId: aws.String(queryID), NextToken: optionalString(nextToken), MaxResults: aws.Int32(1000),
		})
		if err != nil {
			return nil, safeAWSError("read Athena CUR results", err)
		}
		if output.ResultSet == nil {
			return nil, errors.New("Athena CUR query returned no result set")
		}
		for index, row := range output.ResultSet.Rows {
			if firstPage && index == 0 {
				continue
			}
			values := athenaRowValues(row)
			if len(values) < 12 {
				return nil, fmt.Errorf("Athena CUR row has %d columns, expected 12", len(values))
			}
			bucket, normalizeErr := NormalizeCURRow(CURRow{
				AccountID: values[0], PeriodStart: values[1], PeriodEnd: values[2], Region: values[3],
				ModelID: values[4], Operation: values[5], UsageType: values[6], UsageQuantity: values[7],
				UnblendedCost: values[8], NetCost: values[9], Currency: values[10], LineItemID: values[11],
			}, provider.config.Maturity)
			if normalizeErr != nil {
				return nil, normalizeErr
			}
			result = append(result, bucket)
		}
		firstPage = false
		nextToken = aws.ToString(output.NextToken)
		if nextToken == "" {
			break
		}
	}
	return result, nil
}

func (provider *AthenaCURProvider) costQuery(period reconcile.Period) string {
	start := period.Start.UTC().Format(time.RFC3339)
	end := period.End.UTC().Format(time.RFC3339)
	return fmt.Sprintf(`SELECT
line_item_usage_account_id,
date_format(line_item_usage_start_date, '%%Y-%%m-%%dT%%H:%%i:%%sZ'),
date_format(line_item_usage_end_date, '%%Y-%%m-%%dT%%H:%%i:%%sZ'),
product_region_code,
COALESCE(line_item_resource_id, ''),
line_item_operation,
line_item_usage_type,
CAST(SUM(line_item_usage_amount) AS varchar),
CAST(SUM(line_item_unblended_cost) AS varchar),
CAST(SUM(line_item_unblended_cost) AS varchar),
line_item_currency_code,
CONCAT(date_format(line_item_usage_start_date, '%%Y%%m%%d%%H'), '|', line_item_usage_type, '|', COALESCE(line_item_resource_id, ''))
FROM "%s"."%s"
WHERE line_item_product_code = 'AmazonBedrock'
AND line_item_usage_account_id = '%s'
AND product_region_code = '%s'
AND line_item_usage_start_date >= from_iso8601_timestamp('%s')
AND line_item_usage_start_date < from_iso8601_timestamp('%s')
GROUP BY 1,2,3,4,5,6,7,11,12
ORDER BY 2,4,5,6,7`, provider.config.Database, provider.config.Table, provider.config.AccountID, provider.config.Region, start, end)
}

func athenaRowValues(row athenatypes.Row) []string {
	values := make([]string, 0, len(row.Data))
	for _, datum := range row.Data {
		values = append(values, aws.ToString(datum.VarCharValue))
	}
	return values
}

func validateAthenaAccountID(accountID string) bool {
	return len(accountID) == 12 && strings.Trim(accountID, "0123456789") == ""
}

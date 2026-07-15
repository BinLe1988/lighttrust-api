package bedrock

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type cloudWatchMetricsAPI interface {
	GetMetricData(context.Context, *cloudwatch.GetMetricDataInput, ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

type CompletenessSummary struct {
	Invocations      float64 `json:"invocations"`
	InputTokens      float64 `json:"input_tokens"`
	OutputTokens     float64 `json:"output_tokens"`
	CacheReadTokens  float64 `json:"cache_read_tokens"`
	CacheWriteTokens float64 `json:"cache_write_tokens"`
}

type CloudWatchCompletenessProvider struct {
	client cloudWatchMetricsAPI
}

func NewCloudWatchCompletenessProvider(cfg aws.Config) *CloudWatchCompletenessProvider {
	return &CloudWatchCompletenessProvider{client: cloudwatch.NewFromConfig(cfg)}
}

func newCloudWatchCompletenessProvider(client cloudWatchMetricsAPI) *CloudWatchCompletenessProvider {
	return &CloudWatchCompletenessProvider{client: client}
}

func (provider *CloudWatchCompletenessProvider) PullCompleteness(ctx context.Context, period reconcile.Period) (CompletenessSummary, error) {
	if provider == nil || provider.client == nil {
		return CompletenessSummary{}, errors.New("CloudWatch metrics client is required")
	}
	if err := period.Validate(); err != nil {
		return CompletenessSummary{}, err
	}
	queries := []cloudwatchtypes.MetricDataQuery{
		metricSearch("invocations", "Invocations"),
		metricSearch("input_tokens", "InputTokenCount"),
		metricSearch("output_tokens", "OutputTokenCount"),
		metricSearch("cache_read", "CacheReadInputTokens"),
		metricSearch("cache_write", "CacheWriteInputTokens"),
	}
	nextToken := ""
	summary := CompletenessSummary{}
	for {
		output, err := provider.client.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
			StartTime: &period.Start, EndTime: &period.End, MetricDataQueries: queries,
			NextToken: optionalString(nextToken), ScanBy: cloudwatchtypes.ScanByTimestampAscending,
		})
		if err != nil {
			return CompletenessSummary{}, safeAWSError("read CloudWatch Bedrock completeness metrics", err)
		}
		for _, result := range output.MetricDataResults {
			value := sumFloat64(result.Values)
			switch aws.ToString(result.Id) {
			case "invocations":
				summary.Invocations += value
			case "input_tokens":
				summary.InputTokens += value
			case "output_tokens":
				summary.OutputTokens += value
			case "cache_read":
				summary.CacheReadTokens += value
			case "cache_write":
				summary.CacheWriteTokens += value
			}
		}
		nextToken = aws.ToString(output.NextToken)
		if nextToken == "" {
			break
		}
	}
	return summary, nil
}

func metricSearch(id, metricName string) cloudwatchtypes.MetricDataQuery {
	expression := "SUM(SEARCH('{AWS/Bedrock,ModelId} MetricName=\"" + metricName + "\"', 'Sum', 86400))"
	return cloudwatchtypes.MetricDataQuery{Id: aws.String(id), Expression: aws.String(expression), ReturnData: aws.Bool(true)}
}

func sumFloat64(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	return total
}

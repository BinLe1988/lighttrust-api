package bedrock

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCloudWatchMetrics struct {
	input *cloudwatch.GetMetricDataInput
}

func (stub *stubCloudWatchMetrics) GetMetricData(_ context.Context, input *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	stub.input = input
	return &cloudwatch.GetMetricDataOutput{MetricDataResults: []cloudwatchtypes.MetricDataResult{
		{Id: aws.String("invocations"), Values: []float64{2, 3}},
		{Id: aws.String("input_tokens"), Values: []float64{100}},
		{Id: aws.String("output_tokens"), Values: []float64{50}},
	}}, nil
}

func TestCloudWatchCompletenessUsesBedrockMetricSearch(t *testing.T) {
	stub := &stubCloudWatchMetrics{}
	provider := newCloudWatchCompletenessProvider(stub)
	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	summary, err := provider.PullCompleteness(context.Background(), reconcile.Period{Start: day, End: day.Add(24 * time.Hour)})
	require.NoError(t, err)
	assert.Equal(t, float64(5), summary.Invocations)
	assert.Equal(t, float64(100), summary.InputTokens)
	require.Len(t, stub.input.MetricDataQueries, 5)
	assert.Contains(t, aws.ToString(stub.input.MetricDataQueries[0].Expression), "AWS/Bedrock")
	assert.Contains(t, aws.ToString(stub.input.MetricDataQueries[0].Expression), "Invocations")
}

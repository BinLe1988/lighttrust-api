package bedrock

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCloudWatchLogs struct {
	startInput *cloudwatchlogs.StartQueryInput
	results    []*cloudwatchlogs.GetQueryResultsOutput
}

func (stub *stubCloudWatchLogs) StartQuery(_ context.Context, input *cloudwatchlogs.StartQueryInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartQueryOutput, error) {
	stub.startInput = input
	return &cloudwatchlogs.StartQueryOutput{QueryId: aws.String("query-1")}, nil
}

func (stub *stubCloudWatchLogs) GetQueryResults(_ context.Context, _ *cloudwatchlogs.GetQueryResultsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	output := stub.results[0]
	stub.results = stub.results[1:]
	return output, nil
}

func TestCloudWatchInvocationProviderResumesQueryAndParsesMessageOnly(t *testing.T) {
	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	message := `{"schemaType":"ModelInvocationLog","schemaVersion":"1.0","timestamp":"2026-07-15T01:02:03Z","accountId":"123456789012","region":"us-east-1","requestId":"aws-1","operation":"InvokeModel","modelId":"model","requestMetadata":{"lighttrust_request_id":"local-1","channel_id":"42"},"input":{"inputTokenCount":3},"output":{"outputTokenCount":4,"text":"must-not-be-stored"}}`
	stub := &stubCloudWatchLogs{results: []*cloudwatchlogs.GetQueryResultsOutput{
		{Status: cloudwatchtypes.QueryStatusRunning},
		{Status: cloudwatchtypes.QueryStatusComplete, Results: [][]cloudwatchtypes.ResultField{{{Field: aws.String("@message"), Value: aws.String(message)}}}},
	}}
	provider, err := newCloudWatchInvocationProvider(stub, "/aws/bedrock/invocations", reconcile.Period{Start: day, End: day.Add(24 * time.Hour)})
	require.NoError(t, err)

	items, cursor, err := provider.PullInvocations(context.Background(), reconcile.Cursor{})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, day.Unix(), aws.ToInt64(stub.startInput.StartTime))
	assert.Contains(t, cursor.Value, "query-1")

	items, cursor, err = provider.PullInvocations(context.Background(), cursor)
	require.NoError(t, err)
	assert.Empty(t, items)
	items, cursor, err = provider.PullInvocations(context.Background(), cursor)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "local-1", items[0].LocalRequestID)
	assert.NotContains(t, items[0].SourceLocation, "must-not-be-stored")
	assert.Contains(t, cursor.Value, `"complete":true`)
}

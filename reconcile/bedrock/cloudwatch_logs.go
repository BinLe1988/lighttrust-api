package bedrock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

const invocationLogsQuery = "fields @message | sort @timestamp asc"

type cloudWatchLogsAPI interface {
	StartQuery(context.Context, *cloudwatchlogs.StartQueryInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartQueryOutput, error)
	GetQueryResults(context.Context, *cloudwatchlogs.GetQueryResultsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetQueryResultsOutput, error)
}

type CloudWatchInvocationProvider struct {
	client   cloudWatchLogsAPI
	logGroup string
	period   reconcile.Period
}

type cloudWatchCursor struct {
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	QueryID   string `json:"query_id,omitempty"`
	NextToken string `json:"next_token,omitempty"`
	Complete  bool   `json:"complete,omitempty"`
}

func NewCloudWatchInvocationProvider(cfg aws.Config, logGroup string, period reconcile.Period) (*CloudWatchInvocationProvider, error) {
	return newCloudWatchInvocationProvider(cloudwatchlogs.NewFromConfig(cfg), logGroup, period)
}

func newCloudWatchInvocationProvider(client cloudWatchLogsAPI, logGroup string, period reconcile.Period) (*CloudWatchInvocationProvider, error) {
	if client == nil || logGroup == "" {
		return nil, errors.New("CloudWatch Logs client and log group are required")
	}
	if err := period.Validate(); err != nil {
		return nil, err
	}
	return &CloudWatchInvocationProvider{client: client, logGroup: logGroup, period: period}, nil
}

func (provider *CloudWatchInvocationProvider) PullInvocations(ctx context.Context, cursor reconcile.Cursor) ([]reconcile.Invocation, reconcile.Cursor, error) {
	state, err := provider.decodeCursor(cursor)
	if err != nil {
		return nil, cursor, err
	}
	if state.Complete {
		return nil, cursor, nil
	}
	if state.QueryID == "" {
		output, startErr := provider.client.StartQuery(ctx, &cloudwatchlogs.StartQueryInput{
			StartTime: aws.Int64(state.Start), EndTime: aws.Int64(state.End),
			LogGroupName: aws.String(provider.logGroup), QueryString: aws.String(invocationLogsQuery), Limit: aws.Int32(100000),
		})
		if startErr != nil {
			return nil, cursor, safeAWSError("start CloudWatch invocation log query", startErr)
		}
		state.QueryID = aws.ToString(output.QueryId)
		if state.QueryID == "" {
			return nil, cursor, errors.New("start CloudWatch invocation log query returned no query id")
		}
		return nil, encodeCloudWatchCursor(state), nil
	}

	output, queryErr := provider.client.GetQueryResults(ctx, &cloudwatchlogs.GetQueryResultsInput{
		QueryId: aws.String(state.QueryID), NextToken: optionalString(state.NextToken), MaxItems: aws.Int32(10000),
	})
	if queryErr != nil {
		return nil, cursor, safeAWSError("read CloudWatch invocation log query", queryErr)
	}
	switch output.Status {
	case cloudwatchtypes.QueryStatusScheduled, cloudwatchtypes.QueryStatusRunning:
		return nil, encodeCloudWatchCursor(state), nil
	case cloudwatchtypes.QueryStatusComplete:
	default:
		return nil, cursor, fmt.Errorf("CloudWatch invocation log query ended with status %s", output.Status)
	}

	invocations := make([]reconcile.Invocation, 0, len(output.Results))
	for index, fields := range output.Results {
		message := resultField(fields, "@message")
		if message == "" {
			return nil, cursor, fmt.Errorf("CloudWatch invocation log result %d has no message", index)
		}
		parsed, parseErr := ParseInvocationLogRecords([]byte(message), fmt.Sprintf("cloudwatch:%s:%s", provider.logGroup, state.QueryID))
		if parseErr != nil {
			return nil, cursor, parseErr
		}
		invocations = append(invocations, parsed...)
	}
	state.NextToken = aws.ToString(output.NextToken)
	if state.NextToken == "" {
		state.Complete = true
	}
	return invocations, encodeCloudWatchCursor(state), nil
}

func (provider *CloudWatchInvocationProvider) decodeCursor(cursor reconcile.Cursor) (cloudWatchCursor, error) {
	state := cloudWatchCursor{Start: provider.period.Start.Unix(), End: provider.period.End.Unix()}
	if cursor.Value == "" {
		return state, nil
	}
	if err := common.UnmarshalJsonStr(cursor.Value, &state); err != nil {
		return cloudWatchCursor{}, fmt.Errorf("decode CloudWatch invocation cursor: %w", err)
	}
	if state.Start <= 0 || state.End <= state.Start {
		return cloudWatchCursor{}, errors.New("invalid CloudWatch invocation cursor period")
	}
	return state, nil
}

func encodeCloudWatchCursor(state cloudWatchCursor) reconcile.Cursor {
	encoded, _ := common.Marshal(state)
	return reconcile.Cursor{Value: string(encoded), UpdatedAt: time.Now().UTC()}
}

func resultField(fields []cloudwatchtypes.ResultField, name string) string {
	for _, field := range fields {
		if aws.ToString(field.Field) == name {
			return aws.ToString(field.Value)
		}
	}
	return ""
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return aws.String(value)
}

func InvocationCursorComplete(cursor reconcile.Cursor) bool {
	if cursor.Value == "" {
		return false
	}
	var state struct {
		Complete bool `json:"complete"`
	}
	return common.UnmarshalJsonStr(cursor.Value, &state) == nil && state.Complete
}

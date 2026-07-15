package bedrock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInvocationLogRecords(t *testing.T) {
	data := []byte(`{
		"schemaType":"ModelInvocationLog",
		"schemaVersion":"1.0",
		"timestamp":"2026-07-15T10:30:00.123Z",
		"accountId":"123456789012",
		"region":"us-east-1",
		"requestId":"aws-request-1",
		"operation":"InvokeModel",
		"modelId":"us.anthropic.claude-sonnet-4-20250514-v1:0",
		"identity":{"arn":"arn:aws:sts::123456789012:assumed-role/BedrockRole/session"},
		"requestMetadata":{"lighttrust_request_id":"local-request-1","channel_id":"42"},
		"input":{"inputTokenCount":25},
		"output":{"outputTokenCount":150},
		"cacheReadInputTokenCount":100,
		"cacheWriteInputTokenCount":20
	}`)

	records, err := ParseInvocationLogRecords(data, "s3://logs/record.json.gz")
	require.NoError(t, err)
	require.Len(t, records, 1)

	record := records[0]
	assert.Equal(t, "aws-request-1", record.RequestID)
	assert.Equal(t, "local-request-1", record.LocalRequestID)
	assert.Equal(t, 42, record.ChannelID)
	assert.Equal(t, "anthropic.claude-sonnet-4-20250514-v1:0", record.NormalizedModelID)
	assert.Equal(t, int64(25), record.InputTokens)
	assert.Equal(t, int64(150), record.OutputTokens)
	assert.Equal(t, int64(100), record.CacheReadInputTokens)
	assert.Equal(t, int64(20), record.CacheWriteInputTokens)
	assert.NotEmpty(t, record.SourceHash)
}

func TestParseInvocationLogRecordsSupportsJSONLines(t *testing.T) {
	data := []byte("{\"schemaType\":\"ModelInvocationLog\",\"schemaVersion\":\"1.0\",\"timestamp\":\"2026-07-15T10:30:00Z\",\"accountId\":\"1\",\"region\":\"us-east-1\",\"requestId\":\"one\"}\n" +
		"{\"schemaType\":\"ModelInvocationLog\",\"schemaVersion\":\"1.0\",\"timestamp\":\"2026-07-15T10:31:00Z\",\"accountId\":\"1\",\"region\":\"us-east-1\",\"requestId\":\"two\"}\n")

	records, err := ParseInvocationLogRecords(data, "cloudwatch://group")
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestParseInvocationLogRecordsRejectsUnknownSchema(t *testing.T) {
	data := []byte(`{"schemaType":"Other","schemaVersion":"1.0","timestamp":"2026-07-15T10:30:00Z"}`)

	_, err := ParseInvocationLogRecords(data, "source")
	require.ErrorContains(t, err, "unsupported schema type")
}

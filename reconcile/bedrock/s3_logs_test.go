package bedrock

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubS3Logs struct {
	listInput *s3.ListObjectsV2Input
	body      []byte
}

func (stub *stubS3Logs) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	stub.listInput = input
	modified := time.Date(2026, time.July, 15, 2, 0, 0, 0, time.UTC)
	return &s3.ListObjectsV2Output{Contents: []types.Object{{Key: aws.String("logs/record.json.gz"), LastModified: &modified}}}, nil
}

func (stub *stubS3Logs) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(stub.body)), ContentEncoding: aws.String("gzip")}, nil
}

func TestS3InvocationProviderReadsBoundedGzipObjects(t *testing.T) {
	message := []byte(`{"schemaType":"ModelInvocationLog","schemaVersion":"1.0","timestamp":"2026-07-15T02:00:00Z","accountId":"123456789012","region":"us-east-1","requestId":"aws-s3-1","operation":"InvokeModel","modelId":"model","input":{"inputTokenCount":1},"output":{"outputTokenCount":2}}`)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write(message)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	stub := &stubS3Logs{body: compressed.Bytes()}
	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	provider, err := newS3InvocationProvider(stub, "bucket", "logs/", "123456789012", reconcile.Period{Start: day, End: day.Add(24 * time.Hour)})
	require.NoError(t, err)

	items, cursor, err := provider.PullInvocations(context.Background(), reconcile.Cursor{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "aws-s3-1", items[0].RequestID)
	assert.Equal(t, "123456789012", aws.ToString(stub.listInput.ExpectedBucketOwner))
	assert.Contains(t, cursor.Value, `"complete":true`)
}

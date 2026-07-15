package bedrock

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const maxInvocationLogObjectBytes = 64 << 20

type s3LogsAPI interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type S3InvocationProvider struct {
	client  s3LogsAPI
	bucket  string
	prefix  string
	account string
	period  reconcile.Period
}

type s3InvocationCursor struct {
	ContinuationToken string `json:"continuation_token,omitempty"`
	Complete          bool   `json:"complete,omitempty"`
}

func NewS3InvocationProvider(cfg aws.Config, bucket, prefix, account string, period reconcile.Period) (*S3InvocationProvider, error) {
	return newS3InvocationProvider(s3.NewFromConfig(cfg), bucket, prefix, account, period)
}

func newS3InvocationProvider(client s3LogsAPI, bucket, prefix, account string, period reconcile.Period) (*S3InvocationProvider, error) {
	if client == nil || bucket == "" || account == "" {
		return nil, errors.New("S3 client, bucket, and account id are required")
	}
	if err := period.Validate(); err != nil {
		return nil, err
	}
	return &S3InvocationProvider{client: client, bucket: bucket, prefix: prefix, account: account, period: period}, nil
}

func (provider *S3InvocationProvider) PullInvocations(ctx context.Context, cursor reconcile.Cursor) ([]reconcile.Invocation, reconcile.Cursor, error) {
	state, err := decodeS3InvocationCursor(cursor)
	if err != nil {
		return nil, cursor, err
	}
	if state.Complete {
		return nil, cursor, nil
	}
	output, err := provider.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(provider.bucket), Prefix: optionalString(provider.prefix),
		ContinuationToken: optionalString(state.ContinuationToken), MaxKeys: aws.Int32(100),
		ExpectedBucketOwner: aws.String(provider.account),
	})
	if err != nil {
		return nil, cursor, safeAWSError("list S3 invocation logs", err)
	}
	invocations := make([]reconcile.Invocation, 0)
	for _, object := range output.Contents {
		key := aws.ToString(object.Key)
		if key == "" || !objectInPeriod(object.LastModified, provider.period) {
			continue
		}
		parsed, readErr := provider.readObject(ctx, key)
		if readErr != nil {
			return nil, cursor, readErr
		}
		invocations = append(invocations, parsed...)
	}
	state.ContinuationToken = aws.ToString(output.NextContinuationToken)
	if !aws.ToBool(output.IsTruncated) || state.ContinuationToken == "" {
		state.Complete = true
	}
	return invocations, encodeS3InvocationCursor(state), nil
}

func (provider *S3InvocationProvider) readObject(ctx context.Context, key string) ([]reconcile.Invocation, error) {
	output, err := provider.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(provider.bucket), Key: aws.String(key), ExpectedBucketOwner: aws.String(provider.account),
	})
	if err != nil {
		return nil, safeAWSError("read S3 invocation log", err)
	}
	defer output.Body.Close()
	var reader io.Reader = output.Body
	if strings.HasSuffix(strings.ToLower(key), ".gz") || strings.EqualFold(aws.ToString(output.ContentEncoding), "gzip") {
		gzipReader, gzipErr := gzip.NewReader(output.Body)
		if gzipErr != nil {
			return nil, fmt.Errorf("open compressed S3 invocation log: %w", gzipErr)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxInvocationLogObjectBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read S3 invocation log body: %w", err)
	}
	if len(data) > maxInvocationLogObjectBytes {
		return nil, errors.New("S3 invocation log object exceeds size limit")
	}
	return ParseInvocationLogRecords(data, "s3://"+provider.bucket+"/"+key)
}

func objectInPeriod(lastModified *time.Time, period reconcile.Period) bool {
	if lastModified == nil {
		return true
	}
	return !lastModified.Before(period.Start) && lastModified.Before(period.End)
}

func decodeS3InvocationCursor(cursor reconcile.Cursor) (s3InvocationCursor, error) {
	var state s3InvocationCursor
	if cursor.Value == "" {
		return state, nil
	}
	if err := common.UnmarshalJsonStr(cursor.Value, &state); err != nil {
		return state, fmt.Errorf("decode S3 invocation cursor: %w", err)
	}
	return state, nil
}

func encodeS3InvocationCursor(state s3InvocationCursor) reconcile.Cursor {
	encoded, _ := common.Marshal(state)
	return reconcile.Cursor{Value: string(encoded), UpdatedAt: time.Now().UTC()}
}

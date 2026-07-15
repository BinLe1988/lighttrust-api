package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bedrockHTTPClientFunc func(*http.Request) (*http.Response, error)

func (f bedrockHTTPClientFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestDoAwsClientRequest_AppliesRuntimeHeaderOverrideToAnthropicBeta(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	ctx.Set(common.RequestIdKey, "local-request-1")

	info := &relaycommon.RelayInfo{
		OriginModelName:           "claude-3-5-sonnet-20240620",
		IsStream:                  false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"anthropic-beta": "computer-use-2025-01-24",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "access-key|secret-key|us-east-1",
			UpstreamModelName: "claude-3-5-sonnet-20240620",
		},
	}

	requestBody := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
	adaptor := &Adaptor{}

	_, err := doAwsClientRequest(ctx, info, adaptor, requestBody)
	require.NoError(t, err)

	awsReq, ok := adaptor.AwsReq.(*bedrockruntime.InvokeModelInput)
	require.True(t, ok)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(awsReq.Body, &payload))

	anthropicBeta, exists := payload["anthropic_beta"]
	require.True(t, exists)

	values, ok := anthropicBeta.([]any)
	require.True(t, ok)
	require.Equal(t, []any{"computer-use-2025-01-24"}, values)
	require.Len(t, adaptor.AwsOptions, 1)
}

func TestBedrockRequestCorrelationIsSignedAndCapturesUpstreamID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set(common.RequestIdKey, "local-request-2")
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42}}

	options, err := newBedrockInvokeOptions(ctx, info)
	require.NoError(t, err)

	var capturedHeader string
	var capturedAuthorization string
	httpClient := bedrockHTTPClientFunc(func(request *http.Request) (*http.Response, error) {
		capturedHeader = request.Header.Get(bedrockRequestMetadataHeader)
		capturedAuthorization = request.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":     []string{"application/json"},
				"X-Amzn-Requestid": []string{"aws-request-2"},
			},
			Body:    io.NopCloser(strings.NewReader(`{"usage":{"inputTokens":1,"outputTokens":1}}`)),
			Request: request,
		}, nil
	})

	client := bedrockruntime.New(bedrockruntime.Options{
		Region:      "us-east-1",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("access", "secret", "")),
		HTTPClient:  httpClient,
	})
	output, err := client.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("anthropic.claude-3-haiku-20240307-v1:0"),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        []byte(`{"messages":[]}`),
	}, options...)
	require.NoError(t, err)

	var metadata map[string]string
	require.NoError(t, common.Unmarshal([]byte(capturedHeader), &metadata))
	assert.Equal(t, "local-request-2", metadata["lighttrust_request_id"])
	assert.Equal(t, "42", metadata["channel_id"])
	assert.Contains(t, strings.ToLower(capturedAuthorization), "x-amzn-bedrock-request-metadata")

	captureBedrockRequestID(ctx, output.ResultMetadata)
	assert.Equal(t, "aws-request-2", ctx.GetString(common.UpstreamRequestIdKey))
}

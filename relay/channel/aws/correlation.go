package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gin-gonic/gin"
)

const bedrockRequestMetadataHeader = "X-Amzn-Bedrock-Request-Metadata"

func newBedrockInvokeOptions(c *gin.Context, info *relaycommon.RelayInfo) ([]func(*bedrockruntime.Options), error) {
	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		return nil, errors.New("request id is required for Bedrock request metadata")
	}

	metadata, err := common.Marshal(map[string]string{
		"lighttrust_request_id": requestID,
		"channel_id":            fmt.Sprintf("%d", info.ChannelId),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal Bedrock request metadata: %w", err)
	}

	addMetadata := func(stack *middleware.Stack) error {
		setHeader := middleware.FinalizeMiddlewareFunc(
			"AddBedrockRequestMetadata",
			func(ctx context.Context, input middleware.FinalizeInput, next middleware.FinalizeHandler) (
				middleware.FinalizeOutput, middleware.Metadata, error,
			) {
				request, ok := input.Request.(*smithyhttp.Request)
				if !ok {
					return middleware.FinalizeOutput{}, middleware.Metadata{}, errors.New("unexpected Bedrock transport request type")
				}
				request.Header.Set(bedrockRequestMetadataHeader, string(metadata))
				return next.HandleFinalize(ctx, input)
			},
		)
		return stack.Finalize.Insert(setHeader, "Signing", middleware.Before)
	}

	return []func(*bedrockruntime.Options){func(options *bedrockruntime.Options) {
		options.APIOptions = append(options.APIOptions, addMetadata)
	}}, nil
}

func captureBedrockRequestID(c *gin.Context, metadata middleware.Metadata) {
	if requestID, ok := awsmiddleware.GetRequestIDMetadata(metadata); ok && requestID != "" {
		c.Set(common.UpstreamRequestIdKey, requestID)
	}
}

func captureBedrockRequestIDFromError(c *gin.Context, err error) {
	var responseError *awshttp.ResponseError
	if errors.As(err, &responseError) && responseError.ServiceRequestID() != "" {
		c.Set(common.UpstreamRequestIdKey, responseError.ServiceRequestID())
	}
}

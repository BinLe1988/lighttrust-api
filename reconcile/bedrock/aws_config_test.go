package bedrock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAssumeRole struct {
	input *sts.AssumeRoleInput
	err   error
}

func (stub *stubAssumeRole) AssumeRole(_ context.Context, input *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	stub.input = input
	if stub.err != nil {
		return nil, stub.err
	}
	expires := time.Now().Add(time.Hour)
	return &sts.AssumeRoleOutput{Credentials: &types.Credentials{
		AccessKeyId: aws.String("temporary-access"), SecretAccessKey: aws.String("temporary-secret"),
		SessionToken: aws.String("temporary-token"), Expiration: &expires,
	}}, nil
}

func TestAssumedRoleConfigRequiresExternalIDAndCachesCredentials(t *testing.T) {
	client := &stubAssumeRole{}
	role := RoleConfig{RoleARN: "arn:aws:iam::123456789012:role/Reconcile", ExternalID: "external-secret", Region: "us-east-1", SessionName: "node/run with spaces"}
	cfg, err := assumedRoleConfig(context.Background(), aws.Config{Region: "us-west-2"}, client, role)
	require.NoError(t, err)
	_, err = cfg.Credentials.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "external-secret", aws.ToString(client.input.ExternalId))
	assert.Equal(t, "node-run-with-spaces", aws.ToString(client.input.RoleSessionName))
	assert.Equal(t, "us-east-1", cfg.Region)
}

type codedAPIError struct{ message string }

func (err codedAPIError) Error() string                 { return err.message }
func (err codedAPIError) ErrorCode() string             { return "AccessDeniedException" }
func (err codedAPIError) ErrorMessage() string          { return err.message }
func (err codedAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestSafeAWSErrorDoesNotExposeSensitiveMessage(t *testing.T) {
	err := safeAWSError("assume role", codedAPIError{message: "external-secret arn:aws:iam::123:role/private"})
	assert.EqualError(t, err, "assume role failed: AccessDeniedException")
	assert.NotContains(t, err.Error(), "external-secret")
	assert.EqualError(t, safeAWSError("load", errors.New("AWS_SECRET_ACCESS_KEY=value")), "load failed")
}

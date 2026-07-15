package bedrock

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

type RoleConfig struct {
	RoleARN     string
	ExternalID  string
	Region      string
	SessionName string
}

type assumeRoleAPI interface {
	AssumeRole(context.Context, *sts.AssumeRoleInput, ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

func LoadAssumedRoleConfig(ctx context.Context, role RoleConfig) (aws.Config, error) {
	if err := role.Validate(); err != nil {
		return aws.Config{}, err
	}
	base, err := config.LoadDefaultConfig(ctx, config.WithRegion(role.Region))
	if err != nil {
		return aws.Config{}, safeAWSError("load AWS base credentials", err)
	}
	return assumedRoleConfig(ctx, base, sts.NewFromConfig(base), role)
}

func assumedRoleConfig(ctx context.Context, base aws.Config, client assumeRoleAPI, role RoleConfig) (aws.Config, error) {
	if err := role.Validate(); err != nil {
		return aws.Config{}, err
	}
	provider := stscreds.NewAssumeRoleProvider(client, role.RoleARN, func(options *stscreds.AssumeRoleOptions) {
		options.ExternalID = aws.String(role.ExternalID)
		options.RoleSessionName = safeRoleSessionName(role.SessionName)
		options.Duration = time.Hour
	})
	base.Region = role.Region
	base.Credentials = aws.NewCredentialsCache(provider, func(options *aws.CredentialsCacheOptions) {
		options.ExpiryWindow = 5 * time.Minute
	})
	if _, err := base.Credentials.Retrieve(ctx); err != nil {
		return aws.Config{}, safeAWSError("assume reconciliation IAM role", err)
	}
	return base, nil
}

func (role RoleConfig) Validate() error {
	if strings.TrimSpace(role.RoleARN) == "" || strings.TrimSpace(role.ExternalID) == "" {
		return errors.New("AWS role ARN and external id are required")
	}
	if strings.TrimSpace(role.Region) == "" {
		return errors.New("AWS region is required")
	}
	return nil
}

var invalidRoleSessionCharacters = regexp.MustCompile(`[^A-Za-z0-9+=,.@_-]+`)

func safeRoleSessionName(value string) string {
	value = invalidRoleSessionCharacters.ReplaceAllString(strings.TrimSpace(value), "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "lighttrust-reconcile"
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func safeAWSError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		return errors.New(operation + " failed: " + apiError.ErrorCode())
	}
	return errors.New(operation + " failed")
}

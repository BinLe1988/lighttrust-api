package bedrock

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
)

type ProviderConfig struct {
	Role                RoleConfig
	AccountID           string
	InvocationSource    string
	InvocationLogGroup  string
	InvocationS3Bucket  string
	InvocationS3Prefix  string
	Athena              AthenaCURConfig
	CostExplorerEnabled bool
	Period              reconcile.Period
}

type AWSProvider struct {
	invocations  reconcile.InvocationProvider
	costs        reconcile.CostProvider
	adjustments  *CostExplorerProvider
	completeness *CloudWatchCompletenessProvider
}

func NewProvider(ctx context.Context, providerConfig ProviderConfig) (*AWSProvider, error) {
	cfg, err := LoadAssumedRoleConfig(ctx, providerConfig.Role)
	if err != nil {
		return nil, err
	}
	return newProviderFromAWSConfig(cfg, providerConfig)
}

func newProviderFromAWSConfig(cfg aws.Config, providerConfig ProviderConfig) (*AWSProvider, error) {
	var invocations reconcile.InvocationProvider
	var err error
	switch providerConfig.InvocationSource {
	case "cloudwatch":
		invocations, err = NewCloudWatchInvocationProvider(cfg, providerConfig.InvocationLogGroup, providerConfig.Period)
	case "s3":
		invocations, err = NewS3InvocationProvider(cfg, providerConfig.InvocationS3Bucket, providerConfig.InvocationS3Prefix, providerConfig.AccountID, providerConfig.Period)
	default:
		err = errors.New("Bedrock invocation source must be cloudwatch or s3")
	}
	if err != nil {
		return nil, err
	}
	costs, err := NewAthenaCURProvider(cfg, providerConfig.Athena)
	if err != nil {
		return nil, err
	}
	provider := &AWSProvider{
		invocations: invocations, costs: costs,
		completeness: NewCloudWatchCompletenessProvider(cfg),
	}
	if providerConfig.CostExplorerEnabled {
		costExplorerConfig := cfg
		costExplorerConfig.Region = "us-east-1"
		provider.adjustments, err = NewCostExplorerProvider(costExplorerConfig, providerConfig.AccountID)
		if err != nil {
			return nil, err
		}
	}
	return provider, nil
}

func (provider *AWSProvider) PullInvocations(ctx context.Context, cursor reconcile.Cursor) ([]reconcile.Invocation, reconcile.Cursor, error) {
	return provider.invocations.PullInvocations(ctx, cursor)
}

func (provider *AWSProvider) PullDailyCosts(ctx context.Context, period reconcile.Period) ([]reconcile.CostBucket, error) {
	return provider.costs.PullDailyCosts(ctx, period)
}

func (provider *AWSProvider) PullAccountAdjustments(ctx context.Context, period reconcile.Period) ([]reconcile.AccountAdjustment, error) {
	if provider.adjustments == nil {
		return nil, nil
	}
	return provider.adjustments.PullAccountAdjustments(ctx, period)
}

func (provider *AWSProvider) PullCompleteness(ctx context.Context, period reconcile.Period) (CompletenessSummary, error) {
	return provider.completeness.PullCompleteness(ctx, period)
}

func (provider *AWSProvider) CheckAccess(ctx context.Context) []reconcile.AccessDiagnostic {
	end := time.Now().UTC().Truncate(time.Hour)
	period := reconcile.Period{Start: end.Add(-time.Hour), End: end}
	diagnostics := make([]reconcile.AccessDiagnostic, 0, 4)
	_, _, invocationErr := provider.PullInvocations(ctx, reconcile.Cursor{})
	diagnostics = append(diagnostics, accessDiagnostic(reconcile.CapabilityInvocations, invocationErr))
	_, costErr := provider.PullDailyCosts(ctx, period)
	diagnostics = append(diagnostics, accessDiagnostic(reconcile.CapabilityDailyCosts, costErr))
	_, adjustmentErr := provider.PullAccountAdjustments(ctx, period)
	diagnostics = append(diagnostics, accessDiagnostic(reconcile.CapabilityAccountAdjustments, adjustmentErr))
	_, completenessErr := provider.PullCompleteness(ctx, period)
	diagnostics = append(diagnostics, accessDiagnostic(reconcile.CapabilityCompleteness, completenessErr))
	return diagnostics
}

func accessDiagnostic(capability string, err error) reconcile.AccessDiagnostic {
	if err == nil {
		return reconcile.AccessDiagnostic{Capability: capability, Available: true, Message: "available"}
	}
	return reconcile.AccessDiagnostic{Capability: capability, Available: false, Message: err.Error()}
}

var _ reconcile.Provider = (*AWSProvider)(nil)

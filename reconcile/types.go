package reconcile

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	ProviderBedrock = "bedrock"

	CapabilityInvocations        = "invocations"
	CapabilityDailyCosts         = "daily_costs"
	CapabilityAccountAdjustments = "account_adjustments"
	CapabilityCompleteness       = "completeness"
)

type Maturity string

const (
	MaturityPending     Maturity = "pending"
	MaturityProvisional Maturity = "provisional"
	MaturityFinal       Maturity = "final"
)

type MatchMethod string

const (
	MatchMethodRequestMetadata MatchMethod = "request_metadata"
	MatchMethodUpstreamID      MatchMethod = "upstream_request_id"
	MatchMethodSignature       MatchMethod = "signature"
)

type Confidence string

const (
	ConfidenceExact    Confidence = "exact"
	ConfidenceProbable Confidence = "probable"
)

type ItemStatus string

const (
	ItemStatusMatched         ItemStatus = "matched"
	ItemStatusTokenMismatch   ItemStatus = "token_mismatch"
	ItemStatusModelMismatch   ItemStatus = "model_mismatch"
	ItemStatusInternalMissing ItemStatus = "internal_missing"
	ItemStatusUpstreamMissing ItemStatus = "upstream_missing"
	ItemStatusDuplicate       ItemStatus = "duplicate"
	ItemStatusAmbiguous       ItemStatus = "ambiguous"
	ItemStatusPending         ItemStatus = "pending"
)

type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
)

type Period struct {
	Start time.Time
	End   time.Time
}

func (p Period) Validate() error {
	if p.Start.IsZero() || p.End.IsZero() {
		return errors.New("reconciliation period requires start and end")
	}
	if !p.Start.Before(p.End) {
		return errors.New("reconciliation period start must be before end")
	}
	return nil
}

type Cursor struct {
	Value     string
	UpdatedAt time.Time
}

type Invocation struct {
	Provider              string
	AccountID             string
	Region                string
	RequestID             string
	LocalRequestID        string
	ChannelID             int
	Timestamp             time.Time
	Operation             string
	ModelID               string
	NormalizedModelID     string
	InputTokens           int64
	OutputTokens          int64
	CacheReadInputTokens  int64
	CacheWriteInputTokens int64
	IdentityARN           string
	SourceLocation        string
	SourceHash            string
}

func (i Invocation) Validate() error {
	if strings.TrimSpace(i.Provider) == "" {
		return errors.New("invocation provider is required")
	}
	if strings.TrimSpace(i.AccountID) == "" {
		return errors.New("invocation account id is required")
	}
	if strings.TrimSpace(i.Region) == "" {
		return errors.New("invocation region is required")
	}
	if strings.TrimSpace(i.RequestID) == "" {
		return errors.New("invocation request id is required")
	}
	if i.Timestamp.IsZero() {
		return errors.New("invocation timestamp is required")
	}
	if i.InputTokens < 0 || i.OutputTokens < 0 || i.CacheReadInputTokens < 0 || i.CacheWriteInputTokens < 0 {
		return errors.New("invocation token counts cannot be negative")
	}
	return nil
}

type CostBucket struct {
	Provider      string
	AccountID     string
	Period        Period
	Region        string
	ModelID       string
	Operation     string
	UsageType     string
	TokenCategory string
	ServiceTier   string
	RoutingType   string
	UsageQuantity decimal.Decimal
	UnblendedCost decimal.Decimal
	NetCost       decimal.Decimal
	Currency      string
	SourceKey     string
	SourceHash    string
	Maturity      Maturity
}

type AccountAdjustment struct {
	Provider  string
	AccountID string
	Period    Period
	Type      string
	Amount    decimal.Decimal
	Currency  string
	SourceKey string
}

type AccessDiagnostic struct {
	Capability string
	Available  bool
	Message    string
}

type InvocationProvider interface {
	PullInvocations(ctx context.Context, cursor Cursor) ([]Invocation, Cursor, error)
}

type Provider interface {
	InvocationProvider
	PullDailyCosts(ctx context.Context, period Period) ([]CostBucket, error)
	PullAccountAdjustments(ctx context.Context, period Period) ([]AccountAdjustment, error)
	CheckAccess(ctx context.Context) []AccessDiagnostic
}

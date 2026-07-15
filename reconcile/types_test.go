package reconcile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeriodValidate(t *testing.T) {
	start := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		period  Period
		wantErr string
	}{
		{name: "valid", period: Period{Start: start, End: start.Add(24 * time.Hour)}},
		{name: "missing start", period: Period{End: start}, wantErr: "requires start and end"},
		{name: "equal boundaries", period: Period{Start: start, End: start}, wantErr: "start must be before end"},
		{name: "reversed", period: Period{Start: start.Add(time.Hour), End: start}, wantErr: "start must be before end"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.period.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInvocationValidateRejectsBillingUnsafeTokenCounts(t *testing.T) {
	valid := Invocation{
		Provider:  ProviderBedrock,
		AccountID: "123456789012",
		Region:    "us-east-1",
		RequestID: "request-1",
		Timestamp: time.Now(),
	}
	require.NoError(t, valid.Validate())

	invalid := valid
	invalid.CacheReadInputTokens = -1
	require.ErrorContains(t, invalid.Validate(), "cannot be negative")
}

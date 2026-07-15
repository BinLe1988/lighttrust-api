package bedrock

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCostExplorer struct {
	input *costexplorer.GetCostAndUsageInput
}

func (stub *stubCostExplorer) GetCostAndUsage(_ context.Context, input *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	stub.input = input
	return &costexplorer.GetCostAndUsageOutput{ResultsByTime: []cetypes.ResultByTime{{
		TimePeriod: &cetypes.DateInterval{Start: aws.String("2026-07-15"), End: aws.String("2026-07-16")},
		Groups: []cetypes.Group{
			{Keys: []string{"Credit"}, Metrics: map[string]cetypes.MetricValue{"NetUnblendedCost": {Amount: aws.String("-0.125"), Unit: aws.String("USD")}}},
			{Keys: []string{"Usage"}, Metrics: map[string]cetypes.MetricValue{"NetUnblendedCost": {Amount: aws.String("2"), Unit: aws.String("USD")}}},
		},
	}}}, nil
}

func TestCostExplorerProviderReturnsOnlyBillingAdjustments(t *testing.T) {
	stub := &stubCostExplorer{}
	provider, err := newCostExplorerProvider(stub, "123456789012")
	require.NoError(t, err)
	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	adjustments, err := provider.PullAccountAdjustments(context.Background(), reconcile.Period{Start: day, End: day.Add(24 * time.Hour)})
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.Equal(t, reconcile.AdjustmentCredit, adjustments[0].Type)
	assert.True(t, adjustments[0].Amount.Equal(decimal.RequireFromString("-0.125")))
	assert.Equal(t, cetypes.GranularityDaily, stub.input.Granularity)
	assert.Equal(t, "RECORD_TYPE", aws.ToString(stub.input.GroupBy[0].Key))
}

package bedrock

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/reconcile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAthena struct {
	startInput *athena.StartQueryExecutionInput
	result     *athena.GetQueryResultsOutput
}

func (stub *stubAthena) StartQueryExecution(_ context.Context, input *athena.StartQueryExecutionInput, _ ...func(*athena.Options)) (*athena.StartQueryExecutionOutput, error) {
	stub.startInput = input
	return &athena.StartQueryExecutionOutput{QueryExecutionId: aws.String("athena-query-1")}, nil
}

func (stub *stubAthena) GetQueryExecution(_ context.Context, _ *athena.GetQueryExecutionInput, _ ...func(*athena.Options)) (*athena.GetQueryExecutionOutput, error) {
	return &athena.GetQueryExecutionOutput{QueryExecution: &athenatypes.QueryExecution{Status: &athenatypes.QueryExecutionStatus{State: athenatypes.QueryExecutionStateSucceeded}}}, nil
}

func (stub *stubAthena) GetQueryResults(_ context.Context, _ *athena.GetQueryResultsInput, _ ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error) {
	return stub.result, nil
}

func athenaRow(values ...string) athenatypes.Row {
	data := make([]athenatypes.Datum, 0, len(values))
	for _, value := range values {
		data = append(data, athenatypes.Datum{VarCharValue: aws.String(value)})
	}
	return athenatypes.Row{Data: data}
}

func TestAthenaCURProviderUsesIdempotentBoundedQueryAndExactDecimals(t *testing.T) {
	stub := &stubAthena{result: &athena.GetQueryResultsOutput{ResultSet: &athenatypes.ResultSet{Rows: []athenatypes.Row{
		athenaRow("account", "start", "end", "region", "model", "operation", "usage", "quantity", "unblended", "net", "currency", "id"),
		athenaRow("123456789012", "2026-07-15T00:00:00Z", "2026-07-15T01:00:00Z", "us-east-1", "model", "InvokeModel", "USE1-input-token", "100.125", "0.000000001", "0.000000001", "USD", "line-1"),
	}}}}
	provider, err := newAthenaCURProvider(stub, AthenaCURConfig{
		Database: "cur_db", Table: "cur_table", Workgroup: "reconcile", OutputLocation: "s3://query-results/prefix/",
		AccountID: "123456789012", Region: "us-east-1", Maturity: reconcile.MaturityFinal,
	})
	require.NoError(t, err)
	day := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	buckets, err := provider.PullDailyCosts(context.Background(), reconcile.Period{Start: day, End: day.Add(24 * time.Hour)})
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	assert.True(t, buckets[0].UsageQuantity.Equal(decimal.RequireFromString("100.125")))
	assert.True(t, buckets[0].NetCost.Equal(decimal.RequireFromString("0.000000001")))
	assert.Contains(t, aws.ToString(stub.startInput.QueryString), "line_item_product_code = 'AmazonBedrock'")
	assert.Contains(t, aws.ToString(stub.startInput.QueryString), "2026-07-15T00:00:00Z")
	assert.NotEmpty(t, aws.ToString(stub.startInput.ClientRequestToken))
}

func TestAthenaCURProviderRejectsUnsafeIdentifiersAndAccount(t *testing.T) {
	_, err := newAthenaCURProvider(&stubAthena{}, AthenaCURConfig{Database: "db;DROP", Table: "table", Workgroup: "wg", OutputLocation: "s3://out", AccountID: "123456789012", Region: "us-east-1"})
	assert.ErrorContains(t, err, "safe SQL identifiers")
	_, err = newAthenaCURProvider(&stubAthena{}, AthenaCURConfig{Database: "db", Table: "table", Workgroup: "wg", OutputLocation: "s3://out", AccountID: "123' OR 1=1", Region: "us-east-1"})
	assert.ErrorContains(t, err, "12 digits")
}

package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubInvocationProvider struct {
	invocations []reconcile.Invocation
	nextCursor  reconcile.Cursor
	err         error
}

func (s stubInvocationProvider) PullInvocations(
	_ context.Context,
	_ reconcile.Cursor,
) ([]reconcile.Invocation, reconcile.Cursor, error) {
	return s.invocations, s.nextCursor, s.err
}

func TestIngestReconcileInvocationsPersistsCursorPageIdempotently(t *testing.T) {
	require.NoError(t, model.DB.Exec("DELETE FROM upstream_invocations").Error)
	t.Cleanup(func() {
		require.NoError(t, model.DB.Exec("DELETE FROM upstream_invocations").Error)
	})

	nextCursor := reconcile.Cursor{Value: "next", UpdatedAt: time.Now()}
	provider := stubInvocationProvider{
		nextCursor: nextCursor,
		invocations: []reconcile.Invocation{{
			Provider:     reconcile.ProviderBedrock,
			AccountID:    "123456789012",
			Region:       "us-east-1",
			RequestID:    "aws-request-1",
			Timestamp:    time.Now(),
			InputTokens:  10,
			OutputTokens: 20,
			SourceHash:   "hash",
		}},
	}

	for range 2 {
		counters, cursor, err := IngestReconcileInvocations(context.Background(), provider, reconcile.Cursor{}, "run-1")
		require.NoError(t, err)
		assert.Equal(t, InvocationIngestionCounters{Scanned: 1, Upserted: 1}, counters)
		assert.Equal(t, nextCursor, cursor)
	}

	var count int64
	require.NoError(t, model.DB.Model(&model.UpstreamInvocation{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

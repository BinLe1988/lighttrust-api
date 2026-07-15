package service

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
)

type InvocationIngestionCounters struct {
	Scanned  int `json:"scanned"`
	Upserted int `json:"upserted"`
}

func IngestReconcileInvocations(
	ctx context.Context,
	provider reconcile.InvocationProvider,
	cursor reconcile.Cursor,
	runID string,
) (InvocationIngestionCounters, reconcile.Cursor, error) {
	if provider == nil {
		return InvocationIngestionCounters{}, cursor, errors.New("reconciliation invocation provider is nil")
	}
	if runID == "" {
		return InvocationIngestionCounters{}, cursor, errors.New("reconciliation run id is required")
	}

	invocations, nextCursor, err := provider.PullInvocations(ctx, cursor)
	if err != nil {
		return InvocationIngestionCounters{}, cursor, err
	}
	counters := InvocationIngestionCounters{Scanned: len(invocations)}
	for _, invocation := range invocations {
		if err := invocation.Validate(); err != nil {
			return counters, cursor, err
		}
		record := &model.UpstreamInvocation{
			Provider:              invocation.Provider,
			AccountID:             invocation.AccountID,
			Region:                invocation.Region,
			RequestID:             invocation.RequestID,
			LocalRequestID:        invocation.LocalRequestID,
			ChannelID:             invocation.ChannelID,
			InvokedAt:             invocation.Timestamp.Unix(),
			Operation:             invocation.Operation,
			ModelID:               invocation.ModelID,
			NormalizedModelID:     invocation.NormalizedModelID,
			InputTokens:           invocation.InputTokens,
			OutputTokens:          invocation.OutputTokens,
			CacheReadInputTokens:  invocation.CacheReadInputTokens,
			CacheWriteInputTokens: invocation.CacheWriteInputTokens,
			IdentityARN:           invocation.IdentityARN,
			SourceLocation:        invocation.SourceLocation,
			SourceHash:            invocation.SourceHash,
			IngestionRunID:        runID,
		}
		if err := model.UpsertUpstreamInvocation(record); err != nil {
			return counters, cursor, err
		}
		counters.Upserted++
	}
	return counters, nextCursor, nil
}

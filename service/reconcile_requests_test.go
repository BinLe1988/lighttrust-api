package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearReconcileRequestFixtures(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Exec("DELETE FROM logs").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM upstream_invocations").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM reconcile_items").Error)
	t.Cleanup(func() {
		require.NoError(t, model.DB.Exec("DELETE FROM logs").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM upstream_invocations").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM reconcile_items").Error)
	})
}

func TestReconcileRequestsMatchesBedrockInvocationAndPersistsResult(t *testing.T) {
	clearReconcileRequestFixtures(t)
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	require.NoError(t, model.DB.Create(&model.Log{
		Type:              model.LogTypeConsume,
		RequestId:         "local-request-1",
		UpstreamRequestId: "aws-request-1",
		ChannelId:         42,
		CreatedAt:         now.Unix(),
		ModelName:         "friendly-model-name",
		PromptTokens:      10,
		CompletionTokens:  20,
		Other:             `{"upstream_model_name":"us.anthropic.claude-sonnet-4-20250514-v1:0"}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UpstreamInvocation{
		Provider:          reconcile.ProviderBedrock,
		AccountID:         "123456789012",
		Region:            "us-east-1",
		RequestID:         "aws-request-1",
		LocalRequestID:    "local-request-1",
		ChannelID:         42,
		InvokedAt:         now.Unix(),
		ModelID:           "us.anthropic.claude-sonnet-4-20250514-v1:0",
		NormalizedModelID: "anthropic.claude-sonnet-4-20250514-v1:0",
		InputTokens:       10,
		OutputTokens:      20,
	}).Error)

	counters, err := ReconcileRequests(ReconcileRequestsInput{
		ConfigID:   1,
		AccountID:  "123456789012",
		Regions:    []string{"us-east-1"},
		ChannelIDs: []int{42},
		Period:     reconcile.Period{Start: now.Add(-time.Hour), End: now.Add(time.Hour)},
		Now:        now.Add(time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, counters.Internal)
	assert.Equal(t, 1, counters.Upstream)
	assert.Equal(t, 1, counters.ByStatus[reconcile.ItemStatusMatched])

	var item model.ReconcileItem
	require.NoError(t, model.DB.First(&item).Error)
	assert.Equal(t, string(reconcile.ItemStatusMatched), item.Status)
	assert.Equal(t, string(reconcile.MatchMethodRequestMetadata), item.MatchMethod)
	assert.Equal(t, string(reconcile.ConfidenceExact), item.Confidence)
}

func TestReconcileRequestsKeepsRecentMissingInvocationPending(t *testing.T) {
	clearReconcileRequestFixtures(t)
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	require.NoError(t, model.DB.Create(&model.Log{
		Type:         model.LogTypeConsume,
		RequestId:    "local-request-2",
		ChannelId:    42,
		CreatedAt:    now.Unix(),
		ModelName:    "model",
		PromptTokens: 1,
	}).Error)

	input := ReconcileRequestsInput{
		ConfigID:      1,
		AccountID:     "123456789012",
		Regions:       []string{"us-east-1"},
		ChannelIDs:    []int{42},
		Period:        reconcile.Period{Start: now.Add(-time.Hour), End: now.Add(time.Hour)},
		MaturityDelay: 30 * time.Minute,
		Now:           now.Add(10 * time.Minute),
	}
	counters, err := ReconcileRequests(input)
	require.NoError(t, err)
	assert.Equal(t, 1, counters.ByStatus[reconcile.ItemStatusPending])

	input.Now = now.Add(time.Hour)
	counters, err = ReconcileRequests(input)
	require.NoError(t, err)
	assert.Equal(t, 1, counters.ByStatus[reconcile.ItemStatusUpstreamMissing])

	var count int64
	require.NoError(t, model.DB.Model(&model.ReconcileItem{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

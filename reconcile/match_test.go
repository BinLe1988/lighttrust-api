package reconcile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatcherUsesStablePriorityAndReportsDifferences(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	internal := []InternalInvocation{
		{RequestID: "local-1", UpstreamRequestID: "aws-1", ChannelID: 42, Timestamp: now, ModelID: "model-a", InputTokens: 10, OutputTokens: 20},
		{RequestID: "local-2", UpstreamRequestID: "aws-2", ChannelID: 42, Timestamp: now, ModelID: "model-b", InputTokens: 30, OutputTokens: 40},
	}
	upstream := []Invocation{
		{LocalRequestID: "local-1", RequestID: "different-id", ChannelID: 42, Timestamp: now, ModelID: "model-a", InputTokens: 10, OutputTokens: 20},
		{RequestID: "aws-2", ChannelID: 42, Timestamp: now, ModelID: "model-b", InputTokens: 31, OutputTokens: 40},
	}

	results := NewMatcher(0).Match(internal, upstream)
	require.Len(t, results, 2)
	assert.Equal(t, MatchMethodRequestMetadata, results[0].Method)
	assert.Equal(t, ConfidenceExact, results[0].Confidence)
	assert.Equal(t, ItemStatusMatched, results[0].Status)
	assert.Equal(t, MatchMethodUpstreamID, results[1].Method)
	assert.Equal(t, ItemStatusTokenMismatch, results[1].Status)
}

func TestMatcherMarksRepeatedLocalMetadataAsDuplicate(t *testing.T) {
	now := time.Now()
	internal := []InternalInvocation{{RequestID: "local-1", Timestamp: now, ModelID: "model", InputTokens: 1, OutputTokens: 2}}
	upstream := []Invocation{
		{LocalRequestID: "local-1", RequestID: "aws-1", Timestamp: now, ModelID: "model", InputTokens: 1, OutputTokens: 2},
		{LocalRequestID: "local-1", RequestID: "aws-2", Timestamp: now, ModelID: "model", InputTokens: 1, OutputTokens: 2},
	}

	results := NewMatcher(0).Match(internal, upstream)
	require.Len(t, results, 2)
	assert.Equal(t, ItemStatusDuplicate, results[0].Status)
	assert.Equal(t, ItemStatusDuplicate, results[1].Status)
}

func TestMatcherDoesNotGuessAmbiguousSignature(t *testing.T) {
	now := time.Now()
	internal := []InternalInvocation{
		{RequestID: "one", ChannelID: 1, Timestamp: now, ModelID: "model", InputTokens: 1, OutputTokens: 2},
		{RequestID: "two", ChannelID: 1, Timestamp: now.Add(time.Second), ModelID: "model", InputTokens: 1, OutputTokens: 2},
	}
	upstream := []Invocation{{RequestID: "aws", ChannelID: 1, Timestamp: now, ModelID: "model", InputTokens: 1, OutputTokens: 2}}

	results := NewMatcher(2*time.Minute).Match(internal, upstream)
	require.Len(t, results, 3)
	assert.Equal(t, ItemStatusAmbiguous, results[0].Status)
	assert.Equal(t, -1, results[0].InternalIndex)
	assert.Equal(t, ItemStatusUpstreamMissing, results[1].Status)
	assert.Equal(t, ItemStatusUpstreamMissing, results[2].Status)
}

func TestMatcherReportsBothMissingSides(t *testing.T) {
	now := time.Now()
	results := NewMatcher(0).Match(
		[]InternalInvocation{{RequestID: "internal", Timestamp: now, ModelID: "a"}},
		[]Invocation{{RequestID: "upstream", Timestamp: now, ModelID: "b"}},
	)

	require.Len(t, results, 2)
	assert.Equal(t, ItemStatusInternalMissing, results[0].Status)
	assert.Equal(t, ItemStatusUpstreamMissing, results[1].Status)
}

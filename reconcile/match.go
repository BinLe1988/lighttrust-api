package reconcile

import (
	"strings"
	"time"
)

type InternalInvocation struct {
	RequestID         string
	UpstreamRequestID string
	ChannelID         int
	Timestamp         time.Time
	ModelID           string
	NormalizedModelID string
	InputTokens       int64
	OutputTokens      int64
}

type MatchResult struct {
	InternalIndex int
	UpstreamIndex int
	Method        MatchMethod
	Confidence    Confidence
	Status        ItemStatus
}

type Matcher struct {
	SignatureWindow time.Duration
}

func NewMatcher(signatureWindow time.Duration) Matcher {
	if signatureWindow <= 0 {
		signatureWindow = 2 * time.Minute
	}
	return Matcher{SignatureWindow: signatureWindow}
}

func (m Matcher) Match(internal []InternalInvocation, upstream []Invocation) []MatchResult {
	results := make([]MatchResult, 0, len(internal)+len(upstream))
	matchedInternal := make(map[int]bool, len(internal))
	matchedUpstream := make(map[int]bool, len(upstream))

	internalByRequestID := make(map[string][]int)
	internalByUpstreamID := make(map[string][]int)
	for index, item := range internal {
		if item.RequestID != "" {
			internalByRequestID[item.RequestID] = append(internalByRequestID[item.RequestID], index)
		}
		if item.UpstreamRequestID != "" {
			internalByUpstreamID[item.UpstreamRequestID] = append(internalByUpstreamID[item.UpstreamRequestID], index)
		}
	}

	upstreamByLocalRequestID := make(map[string][]int)
	for index, item := range upstream {
		if item.LocalRequestID != "" {
			upstreamByLocalRequestID[item.LocalRequestID] = append(upstreamByLocalRequestID[item.LocalRequestID], index)
		}
	}

	for localRequestID, upstreamIndexes := range upstreamByLocalRequestID {
		internalIndexes := internalByRequestID[localRequestID]
		if len(upstreamIndexes) > 1 && len(internalIndexes) == 1 {
			for _, upstreamIndex := range upstreamIndexes {
				results = append(results, MatchResult{
					InternalIndex: internalIndexes[0],
					UpstreamIndex: upstreamIndex,
					Method:        MatchMethodRequestMetadata,
					Confidence:    ConfidenceExact,
					Status:        ItemStatusDuplicate,
				})
				matchedUpstream[upstreamIndex] = true
			}
			matchedInternal[internalIndexes[0]] = true
			continue
		}
		if len(upstreamIndexes) != 1 || len(internalIndexes) != 1 {
			continue
		}
		m.appendMatch(&results, matchedInternal, matchedUpstream, internal, upstream,
			internalIndexes[0], upstreamIndexes[0], MatchMethodRequestMetadata, ConfidenceExact)
	}

	for upstreamIndex, upstreamItem := range upstream {
		if matchedUpstream[upstreamIndex] || upstreamItem.RequestID == "" {
			continue
		}
		internalIndexes := unmatchedIndexes(internalByUpstreamID[upstreamItem.RequestID], matchedInternal)
		if len(internalIndexes) != 1 {
			continue
		}
		m.appendMatch(&results, matchedInternal, matchedUpstream, internal, upstream,
			internalIndexes[0], upstreamIndex, MatchMethodUpstreamID, ConfidenceExact)
	}

	for upstreamIndex, upstreamItem := range upstream {
		if matchedUpstream[upstreamIndex] {
			continue
		}
		candidates := make([]int, 0, 1)
		for internalIndex, internalItem := range internal {
			if matchedInternal[internalIndex] || !m.signatureMatches(internalItem, upstreamItem) {
				continue
			}
			candidates = append(candidates, internalIndex)
		}
		if len(candidates) == 1 {
			m.appendMatch(&results, matchedInternal, matchedUpstream, internal, upstream,
				candidates[0], upstreamIndex, MatchMethodSignature, ConfidenceProbable)
			continue
		}
		if len(candidates) > 1 {
			results = append(results, MatchResult{
				InternalIndex: -1,
				UpstreamIndex: upstreamIndex,
				Method:        MatchMethodSignature,
				Confidence:    ConfidenceProbable,
				Status:        ItemStatusAmbiguous,
			})
			matchedUpstream[upstreamIndex] = true
		}
	}

	for upstreamIndex := range upstream {
		if !matchedUpstream[upstreamIndex] {
			results = append(results, MatchResult{
				InternalIndex: -1,
				UpstreamIndex: upstreamIndex,
				Status:        ItemStatusInternalMissing,
			})
		}
	}
	for internalIndex := range internal {
		if !matchedInternal[internalIndex] {
			results = append(results, MatchResult{
				InternalIndex: internalIndex,
				UpstreamIndex: -1,
				Status:        ItemStatusUpstreamMissing,
			})
		}
	}
	return results
}

func (m Matcher) appendMatch(
	results *[]MatchResult,
	matchedInternal map[int]bool,
	matchedUpstream map[int]bool,
	internal []InternalInvocation,
	upstream []Invocation,
	internalIndex int,
	upstreamIndex int,
	method MatchMethod,
	confidence Confidence,
) {
	*results = append(*results, MatchResult{
		InternalIndex: internalIndex,
		UpstreamIndex: upstreamIndex,
		Method:        method,
		Confidence:    confidence,
		Status:        compareMatchedInvocation(internal[internalIndex], upstream[upstreamIndex]),
	})
	matchedInternal[internalIndex] = true
	matchedUpstream[upstreamIndex] = true
}

func (m Matcher) signatureMatches(internal InternalInvocation, upstream Invocation) bool {
	if internal.ChannelID != 0 && upstream.ChannelID != 0 && internal.ChannelID != upstream.ChannelID {
		return false
	}
	if normalizeComparableModel(internal.NormalizedModelID, internal.ModelID) !=
		normalizeComparableModel(upstream.NormalizedModelID, upstream.ModelID) {
		return false
	}
	if internal.InputTokens != upstream.InputTokens || internal.OutputTokens != upstream.OutputTokens {
		return false
	}
	delta := internal.Timestamp.Sub(upstream.Timestamp)
	if delta < 0 {
		delta = -delta
	}
	return delta <= m.SignatureWindow
}

func compareMatchedInvocation(internal InternalInvocation, upstream Invocation) ItemStatus {
	if normalizeComparableModel(internal.NormalizedModelID, internal.ModelID) !=
		normalizeComparableModel(upstream.NormalizedModelID, upstream.ModelID) {
		return ItemStatusModelMismatch
	}
	if internal.InputTokens != upstream.InputTokens || internal.OutputTokens != upstream.OutputTokens {
		return ItemStatusTokenMismatch
	}
	return ItemStatusMatched
}

func normalizeComparableModel(normalized string, fallback string) string {
	if normalized == "" {
		normalized = fallback
	}
	normalized = strings.ToLower(strings.TrimSpace(normalized))
	if slash := strings.LastIndex(normalized, "/"); slash >= 0 {
		normalized = normalized[slash+1:]
	}
	parts := strings.SplitN(normalized, ".", 2)
	if len(parts) == 2 {
		switch parts[0] {
		case "us", "eu", "apac":
			normalized = parts[1]
		}
	}
	return normalized
}

func unmatchedIndexes(indexes []int, matched map[int]bool) []int {
	result := make([]int, 0, len(indexes))
	for _, index := range indexes {
		if !matched[index] {
			result = append(result, index)
		}
	}
	return result
}

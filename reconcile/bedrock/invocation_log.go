package bedrock

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/reconcile"
)

type invocationLogRecord struct {
	SchemaType      string            `json:"schemaType"`
	SchemaVersion   string            `json:"schemaVersion"`
	Timestamp       string            `json:"timestamp"`
	AccountID       string            `json:"accountId"`
	Region          string            `json:"region"`
	RequestID       string            `json:"requestId"`
	Operation       string            `json:"operation"`
	ModelID         string            `json:"modelId"`
	ServiceTier     string            `json:"serviceTier"`
	RequestMetadata map[string]string `json:"requestMetadata"`
	Identity        struct {
		ARN string `json:"arn"`
	} `json:"identity"`
	Input struct {
		InputTokenCount           int64 `json:"inputTokenCount"`
		CacheReadInputTokenCount  int64 `json:"cacheReadInputTokenCount"`
		CacheWriteInputTokenCount int64 `json:"cacheWriteInputTokenCount"`
	} `json:"input"`
	Output struct {
		OutputTokenCount          int64 `json:"outputTokenCount"`
		CacheReadInputTokenCount  int64 `json:"cacheReadInputTokenCount"`
		CacheWriteInputTokenCount int64 `json:"cacheWriteInputTokenCount"`
	} `json:"output"`
	CacheReadInputTokenCount  int64 `json:"cacheReadInputTokenCount"`
	CacheWriteInputTokenCount int64 `json:"cacheWriteInputTokenCount"`
}

func ParseInvocationLogRecords(data []byte, sourceLocation string) ([]reconcile.Invocation, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var rawRecords []invocationLogRecord
	if trimmed[0] == '[' {
		if err := common.Unmarshal(trimmed, &rawRecords); err != nil {
			return nil, fmt.Errorf("decode Bedrock invocation log batch: %w", err)
		}
	} else {
		var record invocationLogRecord
		if err := common.Unmarshal(trimmed, &record); err == nil {
			rawRecords = append(rawRecords, record)
		} else {
			var lineErr error
			scanner := bufio.NewScanner(bytes.NewReader(trimmed))
			scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
			for scanner.Scan() {
				line := bytes.TrimSpace(scanner.Bytes())
				if len(line) == 0 {
					continue
				}
				var lineRecord invocationLogRecord
				if decodeErr := common.Unmarshal(line, &lineRecord); decodeErr != nil {
					lineErr = decodeErr
					break
				}
				rawRecords = append(rawRecords, lineRecord)
			}
			if scanErr := scanner.Err(); scanErr != nil {
				return nil, fmt.Errorf("scan Bedrock invocation log records: %w", scanErr)
			}
			if lineErr != nil {
				return nil, fmt.Errorf("decode Bedrock invocation log record: %w", lineErr)
			}
		}
	}

	invocations := make([]reconcile.Invocation, 0, len(rawRecords))
	for index, record := range rawRecords {
		invocation, err := normalizeInvocationLogRecord(record, sourceLocation)
		if err != nil {
			return nil, fmt.Errorf("normalize Bedrock invocation log record %d: %w", index, err)
		}
		invocations = append(invocations, invocation)
	}
	return invocations, nil
}

func normalizeInvocationLogRecord(record invocationLogRecord, sourceLocation string) (reconcile.Invocation, error) {
	if record.SchemaType != "ModelInvocationLog" {
		return reconcile.Invocation{}, fmt.Errorf("unsupported schema type %q", record.SchemaType)
	}
	if record.SchemaVersion != "1.0" {
		return reconcile.Invocation{}, fmt.Errorf("unsupported schema version %q", record.SchemaVersion)
	}

	invokedAt, err := time.Parse(time.RFC3339Nano, record.Timestamp)
	if err != nil {
		return reconcile.Invocation{}, fmt.Errorf("parse timestamp: %w", err)
	}

	channelID := 0
	if value := record.RequestMetadata["channel_id"]; value != "" {
		channelID, err = strconv.Atoi(value)
		if err != nil || channelID < 0 {
			return reconcile.Invocation{}, fmt.Errorf("invalid channel_id request metadata %q", value)
		}
	}

	cacheReadTokens := record.CacheReadInputTokenCount
	if cacheReadTokens == 0 {
		cacheReadTokens = record.Input.CacheReadInputTokenCount + record.Output.CacheReadInputTokenCount
	}
	cacheWriteTokens := record.CacheWriteInputTokenCount
	if cacheWriteTokens == 0 {
		cacheWriteTokens = record.Input.CacheWriteInputTokenCount + record.Output.CacheWriteInputTokenCount
	}

	sourceBytes, err := common.Marshal(record)
	if err != nil {
		return reconcile.Invocation{}, fmt.Errorf("hash invocation source: %w", err)
	}
	invocation := reconcile.Invocation{
		Provider:              reconcile.ProviderBedrock,
		AccountID:             record.AccountID,
		Region:                record.Region,
		RequestID:             record.RequestID,
		LocalRequestID:        record.RequestMetadata["lighttrust_request_id"],
		ChannelID:             channelID,
		Timestamp:             invokedAt,
		Operation:             record.Operation,
		ModelID:               record.ModelID,
		NormalizedModelID:     normalizeModelID(record.ModelID),
		ServiceTier:           normalizeInvocationServiceTier(record.ServiceTier),
		RoutingType:           invocationRoutingType(record.ModelID),
		InputTokens:           record.Input.InputTokenCount,
		OutputTokens:          record.Output.OutputTokenCount,
		CacheReadInputTokens:  cacheReadTokens,
		CacheWriteInputTokens: cacheWriteTokens,
		IdentityARN:           record.Identity.ARN,
		SourceLocation:        sourceLocation,
		SourceHash:            fmt.Sprintf("%x", common.Sha256Raw(sourceBytes)),
	}
	if err := invocation.Validate(); err != nil {
		return reconcile.Invocation{}, err
	}
	return invocation, nil
}

func normalizeInvocationServiceTier(serviceTier string) string {
	serviceTier = strings.ToLower(strings.TrimSpace(serviceTier))
	if serviceTier == "" || serviceTier == "default" {
		return "standard"
	}
	return serviceTier
}

func invocationRoutingType(modelID string) string {
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	if strings.HasPrefix(modelID, "global.") {
		return "cross_region_global"
	}
	if strings.HasPrefix(modelID, "us.") || strings.HasPrefix(modelID, "eu.") || strings.HasPrefix(modelID, "apac.") {
		return "cross_region"
	}
	return "in_region"
}

func normalizeModelID(modelID string) string {
	normalized := strings.TrimSpace(modelID)
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

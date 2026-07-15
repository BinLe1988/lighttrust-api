package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReconcileInternalLog struct {
	RequestID         string
	UpstreamRequestID string
	ChannelID         int
	CreatedAt         int64
	ModelName         string
	PromptTokens      int
	CompletionTokens  int
	Other             string
}

type ReconcileConfig struct {
	ID                   int64  `json:"id" gorm:"primaryKey"`
	Name                 string `json:"name" gorm:"type:varchar(128);not null"`
	Provider             string `json:"provider" gorm:"type:varchar(32);not null;index"`
	AccountID            string `json:"account_id" gorm:"type:varchar(32);not null;index"`
	RoleARN              string `json:"role_arn" gorm:"type:varchar(512);not null"`
	ExternalID           string `json:"-" gorm:"type:varchar(512);not null"`
	Regions              string `json:"regions" gorm:"type:text;not null"`
	ChannelMappings      string `json:"channel_mappings" gorm:"type:text;not null"`
	InvocationSource     string `json:"invocation_source" gorm:"type:varchar(32);not null"`
	InvocationLogGroup   string `json:"invocation_log_group" gorm:"type:varchar(512)"`
	InvocationS3Bucket   string `json:"invocation_s3_bucket" gorm:"type:varchar(255)"`
	InvocationS3Prefix   string `json:"invocation_s3_prefix" gorm:"type:varchar(1024)"`
	CURS3Bucket          string `json:"cur_s3_bucket" gorm:"type:varchar(255)"`
	CURS3Prefix          string `json:"cur_s3_prefix" gorm:"type:varchar(1024)"`
	AthenaDatabase       string `json:"athena_database" gorm:"type:varchar(255)"`
	AthenaTable          string `json:"athena_table" gorm:"type:varchar(255)"`
	AthenaWorkgroup      string `json:"athena_workgroup" gorm:"type:varchar(255)"`
	AthenaOutputLocation string `json:"athena_output_location" gorm:"type:varchar(1024)"`
	CostExplorerEnabled  bool   `json:"cost_explorer_enabled"`
	Enabled              bool   `json:"enabled"`
	Schedule             string `json:"schedule" gorm:"type:varchar(64)"`
	MaturityDelaySeconds int64  `json:"maturity_delay_seconds" gorm:"bigint"`
	LookbackDays         int    `json:"lookback_days"`
	Tolerance            string `json:"tolerance" gorm:"type:varchar(64)"`
	CreatedAt            int64  `json:"created_at" gorm:"bigint;index;autoCreateTime"`
	UpdatedAt            int64  `json:"updated_at" gorm:"bigint;index;autoUpdateTime"`
}

func ListReconcileConfigs() ([]ReconcileConfig, error) {
	var configs []ReconcileConfig
	err := DB.Order("id asc").Find(&configs).Error
	return configs, err
}

func GetReconcileConfig(id int64) (*ReconcileConfig, error) {
	var config ReconcileConfig
	err := DB.Where("id = ?", id).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &config, err
}

func CreateReconcileConfig(config *ReconcileConfig) error {
	if config == nil {
		return errors.New("reconciliation config is nil")
	}
	if err := config.Validate(); err != nil {
		return err
	}
	return DB.Create(config).Error
}

func UpdateReconcileConfig(config *ReconcileConfig) error {
	if config == nil || config.ID == 0 {
		return errors.New("reconciliation config id is required")
	}
	existing, err := GetReconcileConfig(config.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return gorm.ErrRecordNotFound
	}
	if strings.TrimSpace(config.ExternalID) == "" {
		config.ExternalID = existing.ExternalID
	}
	if err := config.Validate(); err != nil {
		return err
	}
	config.CreatedAt = existing.CreatedAt
	return DB.Save(config).Error
}

func DeleteReconcileConfig(id int64) error {
	if id == 0 {
		return errors.New("reconciliation config id is required")
	}
	return DB.Delete(&ReconcileConfig{}, id).Error
}

type ReconcileRun struct {
	ID               int64  `json:"id" gorm:"primaryKey"`
	RunID            string `json:"run_id" gorm:"type:varchar(64);uniqueIndex"`
	ConfigID         int64  `json:"config_id" gorm:"index;not null"`
	Source           string `json:"source" gorm:"type:varchar(64);index;not null"`
	Status           string `json:"status" gorm:"type:varchar(32);index;not null"`
	Maturity         string `json:"maturity" gorm:"type:varchar(32);index"`
	PeriodStart      int64  `json:"period_start" gorm:"bigint;index"`
	PeriodEnd        int64  `json:"period_end" gorm:"bigint;index"`
	Cursor           string `json:"cursor" gorm:"type:text"`
	QueryExecutionID string `json:"query_execution_id" gorm:"type:varchar(255);index"`
	Counters         string `json:"counters" gorm:"type:text"`
	Error            string `json:"error" gorm:"type:text"`
	LockedBy         string `json:"locked_by" gorm:"type:varchar(128);index"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index;autoCreateTime"`
	UpdatedAt        int64  `json:"updated_at" gorm:"bigint;index;autoUpdateTime"`
}

type UpstreamInvocation struct {
	ID                    int64  `json:"id" gorm:"primaryKey"`
	Provider              string `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:idx_upstream_invocation_identity,priority:1"`
	AccountID             string `json:"account_id" gorm:"type:varchar(32);not null;uniqueIndex:idx_upstream_invocation_identity,priority:2"`
	Region                string `json:"region" gorm:"type:varchar(32);not null;uniqueIndex:idx_upstream_invocation_identity,priority:3"`
	RequestID             string `json:"request_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_upstream_invocation_identity,priority:4"`
	LocalRequestID        string `json:"local_request_id" gorm:"type:varchar(64);index"`
	ChannelID             int    `json:"channel_id" gorm:"index"`
	InvokedAt             int64  `json:"invoked_at" gorm:"bigint;index"`
	Operation             string `json:"operation" gorm:"type:varchar(64);index"`
	ModelID               string `json:"model_id" gorm:"type:varchar(512);index"`
	NormalizedModelID     string `json:"normalized_model_id" gorm:"type:varchar(255);index"`
	ServiceTier           string `json:"service_tier" gorm:"type:varchar(64);index"`
	RoutingType           string `json:"routing_type" gorm:"type:varchar(64);index"`
	InputTokens           int64  `json:"input_tokens" gorm:"bigint"`
	OutputTokens          int64  `json:"output_tokens" gorm:"bigint"`
	CacheReadInputTokens  int64  `json:"cache_read_input_tokens" gorm:"bigint"`
	CacheWriteInputTokens int64  `json:"cache_write_input_tokens" gorm:"bigint"`
	IdentityARN           string `json:"identity_arn" gorm:"type:varchar(1024)"`
	SourceLocation        string `json:"source_location" gorm:"type:text"`
	SourceHash            string `json:"source_hash" gorm:"type:varchar(64);index"`
	IngestionRunID        string `json:"ingestion_run_id" gorm:"type:varchar(64);index"`
	CreatedAt             int64  `json:"created_at" gorm:"bigint;index;autoCreateTime"`
	UpdatedAt             int64  `json:"updated_at" gorm:"bigint;index;autoUpdateTime"`
}

type ReconcileRejectedRecord struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	RunID          string `json:"run_id" gorm:"type:varchar(64);index"`
	Source         string `json:"source" gorm:"type:varchar(64);index"`
	SourceLocation string `json:"source_location" gorm:"type:text"`
	SourceHash     string `json:"source_hash" gorm:"type:varchar(64);index"`
	Reason         string `json:"reason" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index;autoCreateTime"`
}

type UpstreamCostBucket struct {
	ID             int64           `json:"id" gorm:"primaryKey"`
	SourceKey      string          `json:"source_key" gorm:"type:varchar(64);uniqueIndex"`
	Provider       string          `json:"provider" gorm:"type:varchar(32);not null;index"`
	AccountID      string          `json:"account_id" gorm:"type:varchar(32);not null;index"`
	PeriodStart    int64           `json:"period_start" gorm:"bigint;index"`
	PeriodEnd      int64           `json:"period_end" gorm:"bigint;index"`
	Region         string          `json:"region" gorm:"type:varchar(32);index"`
	ModelID        string          `json:"model_id" gorm:"type:varchar(512);index"`
	Operation      string          `json:"operation" gorm:"type:varchar(128);index"`
	UsageType      string          `json:"usage_type" gorm:"type:varchar(255);index"`
	TokenCategory  string          `json:"token_category" gorm:"type:varchar(64);index"`
	ServiceTier    string          `json:"service_tier" gorm:"type:varchar(64);index"`
	RoutingType    string          `json:"routing_type" gorm:"type:varchar(64);index"`
	UsageQuantity  decimal.Decimal `json:"usage_quantity" gorm:"type:text"`
	UnblendedCost  decimal.Decimal `json:"unblended_cost" gorm:"type:text"`
	NetCost        decimal.Decimal `json:"net_cost" gorm:"type:text"`
	Currency       string          `json:"currency" gorm:"type:varchar(8)"`
	Maturity       string          `json:"maturity" gorm:"type:varchar(32);index"`
	SourceHash     string          `json:"source_hash" gorm:"type:varchar(64);index"`
	IngestionRunID string          `json:"ingestion_run_id" gorm:"type:varchar(64);index"`
	CreatedAt      int64           `json:"created_at" gorm:"bigint;index;autoCreateTime"`
	UpdatedAt      int64           `json:"updated_at" gorm:"bigint;index;autoUpdateTime"`
}

func (config *ReconcileConfig) Normalize() {
	config.Name = strings.TrimSpace(config.Name)
	config.Provider = strings.TrimSpace(config.Provider)
	config.AccountID = strings.TrimSpace(config.AccountID)
	config.RoleARN = strings.TrimSpace(config.RoleARN)
	config.ExternalID = strings.TrimSpace(config.ExternalID)
	if config.MaturityDelaySeconds <= 0 {
		config.MaturityDelaySeconds = 30 * 60
	}
	if config.LookbackDays <= 0 {
		config.LookbackDays = 3
	}
	if strings.TrimSpace(config.Tolerance) == "" {
		config.Tolerance = "0"
	}
}

func (config *ReconcileConfig) Validate() error {
	config.Normalize()
	if config.Name == "" || config.Provider == "" || config.AccountID == "" {
		return errors.New("reconciliation name, provider, and account id are required")
	}
	if config.RoleARN == "" || config.ExternalID == "" {
		return errors.New("reconciliation role arn and external id are required")
	}
	if config.Regions == "" || config.ChannelMappings == "" {
		return errors.New("reconciliation regions and channel mappings are required")
	}
	return nil
}

func UpsertUpstreamInvocation(record *UpstreamInvocation) error {
	if record == nil {
		return errors.New("upstream invocation is nil")
	}
	now := common.GetTimestamp()
	if record.CreatedAt == 0 {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider"},
			{Name: "account_id"},
			{Name: "region"},
			{Name: "request_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"local_request_id",
			"channel_id",
			"invoked_at",
			"operation",
			"model_id",
			"normalized_model_id",
			"service_tier",
			"routing_type",
			"input_tokens",
			"output_tokens",
			"cache_read_input_tokens",
			"cache_write_input_tokens",
			"identity_arn",
			"source_location",
			"source_hash",
			"ingestion_run_id",
			"updated_at",
		}),
	}).Create(record).Error
}

func UpsertUpstreamCostBucket(record *UpstreamCostBucket) error {
	if record == nil {
		return errors.New("upstream cost bucket is nil")
	}
	now := common.GetTimestamp()
	if record.CreatedAt == 0 {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider",
			"account_id",
			"period_start",
			"period_end",
			"region",
			"model_id",
			"operation",
			"usage_type",
			"token_category",
			"service_tier",
			"routing_type",
			"usage_quantity",
			"unblended_cost",
			"net_cost",
			"currency",
			"maturity",
			"source_hash",
			"ingestion_run_id",
			"updated_at",
		}),
	}).Create(record).Error
}

func FindInternalLogsForReconcile(channelIDs []int, periodStart int64, periodEnd int64) ([]ReconcileInternalLog, error) {
	if len(channelIDs) == 0 {
		return nil, errors.New("reconciliation channel ids are required")
	}
	if periodStart >= periodEnd {
		return nil, errors.New("invalid reconciliation log period")
	}
	var logs []ReconcileInternalLog
	err := LOG_DB.Model(&Log{}).
		Select("request_id, upstream_request_id, channel_id, created_at, model_name, prompt_tokens, completion_tokens, other").
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeConsume, periodStart, periodEnd).
		Where("channel_id IN ?", channelIDs).
		Where("request_id <> ?", "").
		Order("created_at asc").
		Find(&logs).Error
	return logs, err
}

func FindUpstreamInvocationsForReconcile(
	accountID string,
	regions []string,
	periodStart int64,
	periodEnd int64,
) ([]UpstreamInvocation, error) {
	if accountID == "" {
		return nil, errors.New("reconciliation account id is required")
	}
	if periodStart >= periodEnd {
		return nil, errors.New("invalid reconciliation invocation period")
	}
	query := DB.Where("account_id = ? AND invoked_at >= ? AND invoked_at < ?", accountID, periodStart, periodEnd)
	if len(regions) > 0 {
		query = query.Where("region IN ?", regions)
	}
	var invocations []UpstreamInvocation
	err := query.Order("invoked_at asc").Find(&invocations).Error
	return invocations, err
}

func UpsertReconcileItem(record *ReconcileItem) error {
	if record == nil {
		return errors.New("reconciliation item is nil")
	}
	if record.ItemKey == "" {
		return errors.New("reconciliation item key is required")
	}
	now := common.GetTimestamp()
	if record.FirstObservedAt == 0 {
		record.FirstObservedAt = now
	}
	record.LastObservedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "item_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"config_id",
			"internal_request_id",
			"upstream_invocation_id",
			"match_method",
			"confidence",
			"status",
			"internal_model_id",
			"upstream_model_id",
			"internal_input_tokens",
			"upstream_input_tokens",
			"internal_output_tokens",
			"upstream_output_tokens",
			"upstream_cache_read_tokens",
			"upstream_cache_write_tokens",
			"allocated_cost",
			"cost_kind",
			"currency",
			"maturity",
			"last_observed_at",
			"resolution",
		}),
	}).Create(record).Error
}

func FindCostBucketsForReconcile(
	accountID string,
	regions []string,
	periodStart int64,
	periodEnd int64,
) ([]UpstreamCostBucket, error) {
	if accountID == "" {
		return nil, errors.New("reconciliation account id is required")
	}
	query := DB.Where(
		"account_id = ? AND period_start < ? AND period_end > ?",
		accountID,
		periodEnd,
		periodStart,
	)
	if len(regions) > 0 {
		query = query.Where("region IN ?", regions)
	}
	var buckets []UpstreamCostBucket
	err := query.Order("period_start asc").Find(&buckets).Error
	return buckets, err
}

func UpsertReconcileDailySummary(record *ReconcileDailySummary) error {
	if record == nil || record.SummaryKey == "" {
		return errors.New("reconciliation daily summary key is required")
	}
	record.UpdatedAt = common.GetTimestamp()
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "summary_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"config_id",
			"day",
			"account_id",
			"region",
			"channel_id",
			"model_id",
			"operation",
			"service_tier",
			"routing_type",
			"token_category",
			"internal_requests",
			"upstream_requests",
			"internal_tokens",
			"upstream_tokens",
			"internal_estimated_cost",
			"invocation_log_estimated_cost",
			"cur_cost",
			"absolute_delta",
			"percentage_delta",
			"unmatched_count",
			"maturity",
			"updated_at",
		}),
	}).Create(record).Error
}

type ReconcileItem struct {
	ID                       int64           `json:"id" gorm:"primaryKey"`
	ItemKey                  string          `json:"item_key" gorm:"type:varchar(64);uniqueIndex"`
	ConfigID                 int64           `json:"config_id" gorm:"index"`
	InternalRequestID        string          `json:"internal_request_id" gorm:"type:varchar(64);index"`
	UpstreamInvocationID     int64           `json:"upstream_invocation_id" gorm:"index"`
	MatchMethod              string          `json:"match_method" gorm:"type:varchar(32);index"`
	Confidence               string          `json:"confidence" gorm:"type:varchar(32);index"`
	Status                   string          `json:"status" gorm:"type:varchar(32);index"`
	InternalModelID          string          `json:"internal_model_id" gorm:"type:varchar(255);index"`
	UpstreamModelID          string          `json:"upstream_model_id" gorm:"type:varchar(512);index"`
	InternalInputTokens      int64           `json:"internal_input_tokens" gorm:"bigint"`
	UpstreamInputTokens      int64           `json:"upstream_input_tokens" gorm:"bigint"`
	InternalOutputTokens     int64           `json:"internal_output_tokens" gorm:"bigint"`
	UpstreamOutputTokens     int64           `json:"upstream_output_tokens" gorm:"bigint"`
	UpstreamCacheReadTokens  int64           `json:"upstream_cache_read_tokens" gorm:"bigint"`
	UpstreamCacheWriteTokens int64           `json:"upstream_cache_write_tokens" gorm:"bigint"`
	AllocatedCost            decimal.Decimal `json:"allocated_cost" gorm:"type:text"`
	CostKind                 string          `json:"cost_kind" gorm:"type:varchar(32)"`
	Currency                 string          `json:"currency" gorm:"type:varchar(8)"`
	Maturity                 string          `json:"maturity" gorm:"type:varchar(32);index"`
	FirstObservedAt          int64           `json:"first_observed_at" gorm:"bigint;index"`
	LastObservedAt           int64           `json:"last_observed_at" gorm:"bigint;index"`
	Resolution               string          `json:"resolution" gorm:"type:text"`
}

type ReconcileDailySummary struct {
	ID                         int64           `json:"id" gorm:"primaryKey"`
	SummaryKey                 string          `json:"summary_key" gorm:"type:varchar(64);uniqueIndex"`
	ConfigID                   int64           `json:"config_id" gorm:"index"`
	Day                        int64           `json:"day" gorm:"bigint;index"`
	AccountID                  string          `json:"account_id" gorm:"type:varchar(32);index"`
	Region                     string          `json:"region" gorm:"type:varchar(32);index"`
	ChannelID                  int             `json:"channel_id" gorm:"index"`
	ModelID                    string          `json:"model_id" gorm:"type:varchar(512);index"`
	Operation                  string          `json:"operation" gorm:"type:varchar(128);index"`
	ServiceTier                string          `json:"service_tier" gorm:"type:varchar(64);index"`
	RoutingType                string          `json:"routing_type" gorm:"type:varchar(64);index"`
	TokenCategory              string          `json:"token_category" gorm:"type:varchar(64);index"`
	InternalRequests           int64           `json:"internal_requests" gorm:"bigint"`
	UpstreamRequests           int64           `json:"upstream_requests" gorm:"bigint"`
	InternalTokens             int64           `json:"internal_tokens" gorm:"bigint"`
	UpstreamTokens             int64           `json:"upstream_tokens" gorm:"bigint"`
	InternalEstimatedCost      decimal.Decimal `json:"internal_estimated_cost" gorm:"type:text"`
	InvocationLogEstimatedCost decimal.Decimal `json:"invocation_log_estimated_cost" gorm:"type:text"`
	CURCost                    decimal.Decimal `json:"cur_cost" gorm:"type:text"`
	AbsoluteDelta              decimal.Decimal `json:"absolute_delta" gorm:"type:text"`
	PercentageDelta            decimal.Decimal `json:"percentage_delta" gorm:"type:text"`
	UnmatchedCount             int64           `json:"unmatched_count" gorm:"bigint"`
	Maturity                   string          `json:"maturity" gorm:"type:varchar(32);index"`
	UpdatedAt                  int64           `json:"updated_at" gorm:"bigint;index"`
}

type ReconcileAccountSummary struct {
	ID                int64           `json:"id" gorm:"primaryKey"`
	SummaryKey        string          `json:"summary_key" gorm:"type:varchar(64);uniqueIndex"`
	ConfigID          int64           `json:"config_id" gorm:"index"`
	PeriodStart       int64           `json:"period_start" gorm:"bigint;index"`
	PeriodEnd         int64           `json:"period_end" gorm:"bigint;index"`
	AccountID         string          `json:"account_id" gorm:"type:varchar(32);index"`
	GrossCost         decimal.Decimal `json:"gross_cost" gorm:"type:text"`
	Credits           decimal.Decimal `json:"credits" gorm:"type:text"`
	Refunds           decimal.Decimal `json:"refunds" gorm:"type:text"`
	TaxAndAdjustments decimal.Decimal `json:"tax_and_adjustments" gorm:"type:text"`
	NetCost           decimal.Decimal `json:"net_cost" gorm:"type:text"`
	AttributedCost    decimal.Decimal `json:"attributed_cost" gorm:"type:text"`
	UnattributedCost  decimal.Decimal `json:"unattributed_cost" gorm:"type:text"`
	UnexplainedDelta  decimal.Decimal `json:"unexplained_delta" gorm:"type:text"`
	Currency          string          `json:"currency" gorm:"type:varchar(8)"`
	Maturity          string          `json:"maturity" gorm:"type:varchar(32);index"`
	UpdatedAt         int64           `json:"updated_at" gorm:"bigint;index"`
}

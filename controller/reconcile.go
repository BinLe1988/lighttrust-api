package controller

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
	bedrockreconcile "github.com/QuantumNous/new-api/reconcile/bedrock"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type reconcileConfigRequest struct {
	Name                 string           `json:"name"`
	Provider             string           `json:"provider"`
	AccountID            string           `json:"account_id"`
	RoleARN              string           `json:"role_arn"`
	ExternalID           string           `json:"external_id"`
	Regions              []string         `json:"regions"`
	ChannelMappings      map[string][]int `json:"channel_mappings"`
	InvocationSource     string           `json:"invocation_source"`
	InvocationLogGroup   string           `json:"invocation_log_group"`
	InvocationS3Bucket   string           `json:"invocation_s3_bucket"`
	InvocationS3Prefix   string           `json:"invocation_s3_prefix"`
	CURS3Bucket          string           `json:"cur_s3_bucket"`
	CURS3Prefix          string           `json:"cur_s3_prefix"`
	AthenaDatabase       string           `json:"athena_database"`
	AthenaTable          string           `json:"athena_table"`
	AthenaWorkgroup      string           `json:"athena_workgroup"`
	AthenaOutputLocation string           `json:"athena_output_location"`
	CostExplorerEnabled  bool             `json:"cost_explorer_enabled"`
	Enabled              bool             `json:"enabled"`
	Schedule             string           `json:"schedule"`
	MaturityDelaySeconds int64            `json:"maturity_delay_seconds"`
	LookbackDays         int              `json:"lookback_days"`
	Tolerance            string           `json:"tolerance"`
}

type reconcileConfigResponse struct {
	*model.ReconcileConfig
	Regions              []string         `json:"regions"`
	ChannelMappings      map[string][]int `json:"channel_mappings"`
	ExternalIDConfigured bool             `json:"external_id_configured"`
}

func ListReconcileConfigs(c *gin.Context) {
	configs, err := model.ListReconcileConfigs()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	responses := make([]reconcileConfigResponse, 0, len(configs))
	for index := range configs {
		response, err := makeReconcileConfigResponse(&configs[index])
		if err != nil {
			common.ApiError(c, err)
			return
		}
		responses = append(responses, response)
	}
	common.ApiSuccess(c, responses)
}

func GetReconcileConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid reconciliation config id"})
		return
	}
	config, err := model.GetReconcileConfig(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if config == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "reconciliation config not found"})
		return
	}
	response, err := makeReconcileConfigResponse(config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func CreateReconcileConfig(c *gin.Context) {
	var request reconcileConfigRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	config, err := request.toModel()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.CreateReconcileConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	recordManageAudit(c, "reconcile.config_create", map[string]interface{}{"id": config.ID, "name": config.Name})
	response, _ := makeReconcileConfigResponse(config)
	common.ApiSuccess(c, response)
}

func UpdateReconcileConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid reconciliation config id"})
		return
	}
	var request reconcileConfigRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	config, err := request.toModel()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	config.ID = id
	if err := model.UpdateReconcileConfig(config); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "reconciliation config not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	recordManageAudit(c, "reconcile.config_update", map[string]interface{}{"id": config.ID, "name": config.Name})
	response, _ := makeReconcileConfigResponse(config)
	common.ApiSuccess(c, response)
}

func DeleteReconcileConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid reconciliation config id"})
		return
	}
	if err := model.DeleteReconcileConfig(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "reconcile.config_delete", map[string]interface{}{"id": id})
	common.ApiSuccess(c, nil)
}

func DiagnoseReconcileConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid reconciliation config id"})
		return
	}
	config, err := model.GetReconcileConfig(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if config == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "reconciliation config not found"})
		return
	}
	var regions []string
	if err := common.UnmarshalJsonStr(config.Regions, &regions); err != nil {
		common.ApiError(c, err)
		return
	}
	end := time.Now().UTC().Truncate(time.Hour)
	period := reconcile.Period{Start: end.Add(-time.Hour), End: end}
	results := make(map[string][]reconcile.AccessDiagnostic, len(regions))
	for _, region := range regions {
		providerContext, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		provider, providerErr := bedrockreconcile.NewProvider(providerContext, bedrockProviderConfig(config, region, period, "diagnostic"))
		if providerErr != nil {
			results[region] = []reconcile.AccessDiagnostic{{Capability: "assume_role", Available: false, Message: providerErr.Error()}}
			cancel()
			continue
		}
		results[region] = provider.CheckAccess(providerContext)
		cancel()
	}
	recordManageAudit(c, "reconcile.diagnostic", map[string]interface{}{"id": config.ID, "name": config.Name})
	common.ApiSuccess(c, results)
}

type reconcileRunRequest struct {
	ConfigID    int64 `json:"config_id"`
	PeriodStart int64 `json:"period_start"`
	PeriodEnd   int64 `json:"period_end"`
}

func CreateReconcileRun(c *gin.Context) {
	var request reconcileRunRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	config, period, err := validateReconcileRunRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeBedrockReconcile, service.BedrockReconcileTaskPayload{
		ConfigID: config.ID, PeriodStart: period.Start.Unix(), PeriodEnd: period.End.Unix(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !created {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "a Bedrock reconciliation task is already active", "data": task.ToResponse()})
		return
	}
	recordManageAudit(c, "reconcile.run_create", map[string]interface{}{"id": config.ID, "name": config.Name, "task_id": task.TaskID})
	common.ApiSuccess(c, task.ToResponse())
}

func RetryReconcileRun(c *gin.Context) {
	run, err := model.GetReconcileRun(c.Param("run_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if run == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "reconciliation run not found"})
		return
	}
	request := reconcileRunRequest{ConfigID: run.ConfigID, PeriodStart: run.PeriodStart, PeriodEnd: run.PeriodEnd}
	config, period, err := validateReconcileRunRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeBedrockReconcile, service.BedrockReconcileTaskPayload{
		ConfigID: config.ID, PeriodStart: period.Start.Unix(), PeriodEnd: period.End.Unix(), ResumeRunID: run.RunID,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !created {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "a Bedrock reconciliation task is already active", "data": task.ToResponse()})
		return
	}
	recordManageAudit(c, "reconcile.run_retry", map[string]interface{}{"run_id": run.RunID, "task_id": task.TaskID})
	common.ApiSuccess(c, task.ToResponse())
}

func validateReconcileRunRequest(request reconcileRunRequest) (*model.ReconcileConfig, reconcile.Period, error) {
	if request.ConfigID <= 0 {
		return nil, reconcile.Period{}, errors.New("config_id is required")
	}
	config, err := model.GetReconcileConfig(request.ConfigID)
	if err != nil {
		return nil, reconcile.Period{}, err
	}
	if config == nil {
		return nil, reconcile.Period{}, errors.New("reconciliation config not found")
	}
	period := reconcile.Period{Start: time.Unix(request.PeriodStart, 0), End: time.Unix(request.PeriodEnd, 0)}
	if request.PeriodStart == 0 || request.PeriodEnd == 0 {
		period = service.DefaultReconcilePeriod(config, time.Now())
	}
	if err := period.Validate(); err != nil {
		return nil, reconcile.Period{}, err
	}
	if period.End.Sub(period.Start) > 31*24*time.Hour {
		return nil, reconcile.Period{}, errors.New("reconciliation period cannot exceed 31 days")
	}
	return config, period, nil
}

func bedrockProviderConfig(config *model.ReconcileConfig, region string, period reconcile.Period, runID string) bedrockreconcile.ProviderConfig {
	return bedrockreconcile.ProviderConfig{
		Role: bedrockreconcile.RoleConfig{
			RoleARN: config.RoleARN, ExternalID: config.ExternalID, Region: region,
			SessionName: "lighttrust-" + common.NodeName + "-" + runID,
		},
		AccountID:           config.AccountID,
		InvocationSource:    config.InvocationSource,
		InvocationLogGroup:  config.InvocationLogGroup,
		InvocationS3Bucket:  config.InvocationS3Bucket,
		InvocationS3Prefix:  config.InvocationS3Prefix,
		CostExplorerEnabled: config.CostExplorerEnabled,
		Period:              period,
		Athena: bedrockreconcile.AthenaCURConfig{
			Database: config.AthenaDatabase, Table: config.AthenaTable, Workgroup: config.AthenaWorkgroup,
			OutputLocation: config.AthenaOutputLocation, AccountID: config.AccountID, Region: region, Maturity: reconcile.MaturityProvisional,
		},
	}
}

func ListReconcileItems(c *gin.Context) {
	filter, pageInfo, ok := reconcileResultFilter(c)
	if !ok {
		return
	}
	items, total, err := model.ListReconcileItems(filter)
	writeReconcilePage(c, pageInfo, items, total, err)
}

func ListReconcileDailySummaries(c *gin.Context) {
	filter, pageInfo, ok := reconcileResultFilter(c)
	if !ok {
		return
	}
	summaries, total, err := model.ListReconcileDailySummaries(filter)
	writeReconcilePage(c, pageInfo, summaries, total, err)
}

func ListReconcileAccountSummaries(c *gin.Context) {
	filter, pageInfo, ok := reconcileResultFilter(c)
	if !ok {
		return
	}
	summaries, total, err := model.ListReconcileAccountSummaries(filter)
	writeReconcilePage(c, pageInfo, summaries, total, err)
}

func ListReconcileRuns(c *gin.Context) {
	filter, pageInfo, ok := reconcileResultFilter(c)
	if !ok {
		return
	}
	runs, total, err := model.ListReconcileRuns(filter)
	writeReconcilePage(c, pageInfo, runs, total, err)
}

func reconcileResultFilter(c *gin.Context) (model.ReconcileResultFilter, *common.PageInfo, bool) {
	configID, err := strconv.ParseInt(c.Query("config_id"), 10, 64)
	if err != nil || configID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "config_id is required"})
		return model.ReconcileResultFilter{}, nil, false
	}
	pageInfo := common.GetPageQuery(c)
	channelID, _ := strconv.Atoi(c.Query("channel_id"))
	start, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	return model.ReconcileResultFilter{
		ConfigID:  configID,
		Status:    strings.TrimSpace(c.Query("status")),
		Maturity:  strings.TrimSpace(c.Query("maturity")),
		RequestID: strings.TrimSpace(c.Query("request_id")),
		ModelID:   strings.TrimSpace(c.Query("model_id")),
		Region:    strings.TrimSpace(c.Query("region")),
		ChannelID: channelID,
		Start:     start,
		End:       end,
		Offset:    pageInfo.GetStartIdx(),
		Limit:     pageInfo.GetPageSize(),
	}, pageInfo, true
}

func writeReconcilePage(c *gin.Context, pageInfo *common.PageInfo, items any, total int64, err error) {
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

const maxSynchronousReconcileExportRows = 10000

func ExportReconcileCSV(c *gin.Context) {
	filter, _, ok := reconcileResultFilter(c)
	if !ok {
		return
	}
	kind := c.Query("type")
	filter.Offset = 0
	filter.Limit = 200
	var rows [][]string
	var err error
	switch kind {
	case "items":
		rows, err = exportReconcileItems(filter)
	case "daily":
		rows, err = exportReconcileDaily(filter)
	case "accounts":
		rows, err = exportReconcileAccounts(filter)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "type must be items, daily, or accounts"})
		return
	}
	if err != nil {
		if errors.Is(err, errReconcileExportTooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": err.Error()})
			return
		}
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "reconcile.export", map[string]interface{}{"config_id": filter.ConfigID, "type": kind, "count": len(rows) - 1})
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="reconciliation-%s-%d.csv"`, kind, filter.ConfigID))
	writer := csv.NewWriter(c.Writer)
	writer.WriteAll(rows)
}

var errReconcileExportTooLarge = errors.New("reconciliation export exceeds the 10000 row synchronous limit")

func exportReconcileItems(filter model.ReconcileResultFilter) ([][]string, error) {
	rows := [][]string{{"request_id", "status", "match_method", "confidence", "internal_model", "upstream_model", "internal_input_tokens", "upstream_input_tokens", "internal_output_tokens", "upstream_output_tokens", "cache_read_tokens", "cache_write_tokens", "maturity"}}
	for {
		items, total, err := model.ListReconcileItems(filter)
		if err != nil {
			return nil, err
		}
		if total > maxSynchronousReconcileExportRows {
			return nil, errReconcileExportTooLarge
		}
		for _, item := range items {
			rows = append(rows, []string{item.InternalRequestID, item.Status, item.MatchMethod, item.Confidence, item.InternalModelID, item.UpstreamModelID,
				strconv.FormatInt(item.InternalInputTokens, 10), strconv.FormatInt(item.UpstreamInputTokens, 10), strconv.FormatInt(item.InternalOutputTokens, 10),
				strconv.FormatInt(item.UpstreamOutputTokens, 10), strconv.FormatInt(item.UpstreamCacheReadTokens, 10), strconv.FormatInt(item.UpstreamCacheWriteTokens, 10), item.Maturity})
		}
		filter.Offset += len(items)
		if len(items) == 0 || int64(filter.Offset) >= total {
			return rows, nil
		}
	}
}

func exportReconcileDaily(filter model.ReconcileResultFilter) ([][]string, error) {
	rows := [][]string{{"day", "account_id", "region", "channel_id", "model_id", "operation", "service_tier", "routing_type", "token_category", "upstream_requests", "upstream_tokens", "cur_cost", "absolute_delta", "percentage_delta", "maturity"}}
	for {
		items, total, err := model.ListReconcileDailySummaries(filter)
		if err != nil {
			return nil, err
		}
		if total > maxSynchronousReconcileExportRows {
			return nil, errReconcileExportTooLarge
		}
		for _, item := range items {
			rows = append(rows, []string{strconv.FormatInt(item.Day, 10), item.AccountID, item.Region, strconv.Itoa(item.ChannelID), item.ModelID, item.Operation,
				item.ServiceTier, item.RoutingType, item.TokenCategory, strconv.FormatInt(item.UpstreamRequests, 10), strconv.FormatInt(item.UpstreamTokens, 10),
				item.CURCost.String(), item.AbsoluteDelta.String(), item.PercentageDelta.String(), item.Maturity})
		}
		filter.Offset += len(items)
		if len(items) == 0 || int64(filter.Offset) >= total {
			return rows, nil
		}
	}
}

func exportReconcileAccounts(filter model.ReconcileResultFilter) ([][]string, error) {
	rows := [][]string{{"period_start", "period_end", "account_id", "gross_cost", "credits", "refunds", "tax_and_adjustments", "net_cost", "attributed_cost", "unattributed_cost", "unexplained_delta", "currency", "maturity"}}
	for {
		items, total, err := model.ListReconcileAccountSummaries(filter)
		if err != nil {
			return nil, err
		}
		if total > maxSynchronousReconcileExportRows {
			return nil, errReconcileExportTooLarge
		}
		for _, item := range items {
			rows = append(rows, []string{strconv.FormatInt(item.PeriodStart, 10), strconv.FormatInt(item.PeriodEnd, 10), item.AccountID,
				item.GrossCost.String(), item.Credits.String(), item.Refunds.String(), item.TaxAndAdjustments.String(), item.NetCost.String(),
				item.AttributedCost.String(), item.UnattributedCost.String(), item.UnexplainedDelta.String(), item.Currency, item.Maturity})
		}
		filter.Offset += len(items)
		if len(items) == 0 || int64(filter.Offset) >= total {
			return rows, nil
		}
	}
}

func (request reconcileConfigRequest) toModel() (*model.ReconcileConfig, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.AccountID = strings.TrimSpace(request.AccountID)
	request.RoleARN = strings.TrimSpace(request.RoleARN)
	request.ExternalID = strings.TrimSpace(request.ExternalID)
	if request.Provider == "" {
		request.Provider = reconcile.ProviderBedrock
	}
	if request.Provider != reconcile.ProviderBedrock {
		return nil, errors.New("only the bedrock reconciliation provider is supported")
	}
	if request.InvocationSource != "cloudwatch" && request.InvocationSource != "s3" {
		return nil, errors.New("invocation_source must be cloudwatch or s3")
	}
	if len(request.AccountID) != 12 || strings.Trim(request.AccountID, "0123456789") != "" {
		return nil, errors.New("account_id must contain exactly 12 digits")
	}
	if !strings.HasPrefix(request.RoleARN, "arn:") || !strings.Contains(request.RoleARN, ":iam::"+request.AccountID+":role/") {
		return nil, errors.New("role_arn must be an IAM role in the configured account")
	}
	if request.InvocationSource == "cloudwatch" && strings.TrimSpace(request.InvocationLogGroup) == "" {
		return nil, errors.New("invocation_log_group is required for CloudWatch")
	}
	if request.InvocationSource == "s3" && strings.TrimSpace(request.InvocationS3Bucket) == "" {
		return nil, errors.New("invocation_s3_bucket is required for S3")
	}
	if request.AthenaDatabase == "" || request.AthenaTable == "" || request.AthenaWorkgroup == "" || !strings.HasPrefix(request.AthenaOutputLocation, "s3://") {
		return nil, errors.New("Athena database, table, workgroup, and s3 output location are required")
	}
	if len(request.Regions) == 0 || len(request.ChannelMappings) == 0 {
		return nil, errors.New("regions and channel_mappings are required")
	}
	regionSet := make(map[string]bool, len(request.Regions))
	for _, region := range request.Regions {
		region = strings.TrimSpace(region)
		if region == "" {
			return nil, errors.New("regions cannot contain an empty value")
		}
		regionSet[region] = true
	}
	for region, channelIDs := range request.ChannelMappings {
		if !regionSet[region] || len(channelIDs) == 0 {
			return nil, errors.New("each channel mapping must target a configured region and contain channels")
		}
		for _, channelID := range channelIDs {
			if channelID <= 0 {
				return nil, errors.New("channel ids must be positive")
			}
		}
	}
	allChannelIDs := make([]int, 0)
	for _, channelIDs := range request.ChannelMappings {
		allChannelIDs = append(allChannelIDs, channelIDs...)
	}
	channels, err := model.GetChannelsByIds(allChannelIDs)
	if err != nil {
		return nil, err
	}
	validChannels := make(map[int]bool, len(channels))
	for _, channel := range channels {
		if channel.Type != constant.ChannelTypeAws {
			return nil, fmt.Errorf("channel %d is not an AWS Bedrock channel", channel.Id)
		}
		validChannels[channel.Id] = true
	}
	for _, channelID := range allChannelIDs {
		if !validChannels[channelID] {
			return nil, fmt.Errorf("channel %d does not exist", channelID)
		}
	}
	regions, err := common.Marshal(request.Regions)
	if err != nil {
		return nil, err
	}
	mappings, err := common.Marshal(request.ChannelMappings)
	if err != nil {
		return nil, err
	}
	return &model.ReconcileConfig{
		Name:                 request.Name,
		Provider:             request.Provider,
		AccountID:            request.AccountID,
		RoleARN:              request.RoleARN,
		ExternalID:           request.ExternalID,
		Regions:              string(regions),
		ChannelMappings:      string(mappings),
		InvocationSource:     request.InvocationSource,
		InvocationLogGroup:   request.InvocationLogGroup,
		InvocationS3Bucket:   request.InvocationS3Bucket,
		InvocationS3Prefix:   request.InvocationS3Prefix,
		CURS3Bucket:          request.CURS3Bucket,
		CURS3Prefix:          request.CURS3Prefix,
		AthenaDatabase:       request.AthenaDatabase,
		AthenaTable:          request.AthenaTable,
		AthenaWorkgroup:      request.AthenaWorkgroup,
		AthenaOutputLocation: request.AthenaOutputLocation,
		CostExplorerEnabled:  request.CostExplorerEnabled,
		Enabled:              request.Enabled,
		Schedule:             request.Schedule,
		MaturityDelaySeconds: request.MaturityDelaySeconds,
		LookbackDays:         request.LookbackDays,
		Tolerance:            request.Tolerance,
	}, nil
}

func makeReconcileConfigResponse(config *model.ReconcileConfig) (reconcileConfigResponse, error) {
	var regions []string
	if err := common.UnmarshalJsonStr(config.Regions, &regions); err != nil {
		return reconcileConfigResponse{}, err
	}
	var mappings map[string][]int
	if err := common.UnmarshalJsonStr(config.ChannelMappings, &mappings); err != nil {
		return reconcileConfigResponse{}, err
	}
	copy := *config
	copy.Regions = ""
	copy.ChannelMappings = ""
	return reconcileConfigResponse{
		ReconcileConfig:      &copy,
		Regions:              regions,
		ChannelMappings:      mappings,
		ExternalIDConfigured: config.ExternalID != "",
	}, nil
}

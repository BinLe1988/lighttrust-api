package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/reconcile"
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

func (request reconcileConfigRequest) toModel() (*model.ReconcileConfig, error) {
	if request.Provider == "" {
		request.Provider = reconcile.ProviderBedrock
	}
	if request.Provider != reconcile.ProviderBedrock {
		return nil, errors.New("only the bedrock reconciliation provider is supported")
	}
	if request.InvocationSource != "cloudwatch" && request.InvocationSource != "s3" {
		return nil, errors.New("invocation_source must be cloudwatch or s3")
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

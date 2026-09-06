package helper

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type normalizedTaskBillingRequest struct {
	Model             string `json:"model"`
	Operation         string `json:"operation,omitempty"`
	Resolution        string `json:"resolution,omitempty"`
	Duration          int    `json:"duration,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Sound             *bool  `json:"sound,omitempty"`
	HasReferenceVideo *bool  `json:"has_reference_video,omitempty"`
	HasVoice          *bool  `json:"has_voice,omitempty"`
}

// ModelPriceHelperTask keeps legacy Task/MJ per-call pricing isolated while
// allowing request-aware expressions to opt in through per_request() or task_tokens().
func ModelPriceHelperTask(c *gin.Context, info *relaycommon.RelayInfo) (types.PriceData, error) {
	if info == nil {
		return types.PriceData{}, fmt.Errorf("relay info is required")
	}
	if info.TieredBillingSnapshot != nil {
		return info.PriceData, nil
	}
	billingMode, exprStr, hasExpr := billing_setting.GetModelBillingConfig(info.OriginModelName)
	if billingMode != billing_setting.BillingModeTieredExpr {
		return ModelPriceHelperPerCall(c, info)
	}
	if !hasExpr || strings.TrimSpace(exprStr) == "" {
		return types.PriceData{}, fmt.Errorf("model %s is configured as tiered_expr but has no billing expression", info.OriginModelName)
	}
	if _, err := billingexpr.CompileFromCache(exprStr); err != nil {
		return types.PriceData{}, fmt.Errorf("model %s tiered expr compile failed: %w", info.OriginModelName, err)
	}
	usedVars := billingexpr.UsedVars(exprStr)
	if info.TaskRelayInfo != nil && info.Action == constant.TaskActionRemix {
		if usedVars["per_request"] || usedVars["task_tokens"] {
			return types.PriceData{}, fmt.Errorf("remix is not supported for expression-billed task models")
		}
	}
	switch {
	case usedVars["task_tokens"]:
		return modelPriceHelperTaskTokens(c, info, exprStr)
	case usedVars["per_request"]:
		return modelPriceHelperTaskTiered(c, info, exprStr)
	default:
		return ModelPriceHelperPerCall(c, info)
	}
}

func buildTaskBillingRequestInput(c *gin.Context, info *relaycommon.RelayInfo) (billingexpr.RequestInput, error) {
	taskRequest, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return billingexpr.RequestInput{}, fmt.Errorf("resolve normalized task billing request: %w", err)
	}
	duration, err := relaycommon.ResolveTaskDuration(taskRequest)
	if err != nil {
		return billingexpr.RequestInput{}, fmt.Errorf("resolve normalized task billing duration: %w", err)
	}

	normalizedRequest := normalizedTaskBillingRequest{
		Model:      info.OriginModelName,
		Resolution: taskRequest.Resolution,
		Duration:   duration,
		Mode:       taskRequest.Mode,
	}
	if info.ChannelType != constant.ChannelTypeBaiduV2 {
		hasReferenceVideo := taskRequest.HasReferenceVideo
		normalizedRequest.HasReferenceVideo = &hasReferenceVideo
	}
	if info.ChannelType == constant.ChannelTypeBaiduV2 && taskRequest.Metadata != nil {
		if operation, ok := taskRequest.Metadata["qianfan_operation"].(string); ok {
			normalizedRequest.Operation = operation
		}
		if sound, ok := taskRequest.Metadata["qianfan_sound"].(bool); ok {
			normalizedRequest.Sound = &sound
		}
		if hasReferenceVideo, ok := taskRequest.Metadata["qianfan_has_reference_video"].(bool); ok {
			normalizedRequest.HasReferenceVideo = &hasReferenceVideo
		}
		if hasVoice, ok := taskRequest.Metadata["qianfan_has_voice"].(bool); ok {
			normalizedRequest.HasVoice = &hasVoice
		}
	}

	requestInput, err := BuildBillingExprRequestInputFromRequest(normalizedRequest, info.RequestHeaders)
	if err != nil {
		return billingexpr.RequestInput{}, fmt.Errorf("build normalized task billing request: %w", err)
	}
	return requestInput, nil
}

func modelPriceHelperTaskTiered(c *gin.Context, info *relaycommon.RelayInfo, exprStr string) (types.PriceData, error) {
	requestInput, err := buildTaskBillingRequestInput(c, info)
	if err != nil {
		return types.PriceData{}, err
	}

	rawCost, trace, err := billingexpr.RunExprWithRequest(exprStr, billingexpr.TokenParams{}, requestInput)
	if err != nil {
		return types.PriceData{}, fmt.Errorf("model %s tiered task expr run failed: %w", info.OriginModelName, err)
	}
	if math.IsNaN(rawCost) || math.IsInf(rawCost, 0) || rawCost < 0 {
		return types.PriceData{}, fmt.Errorf("model %s tiered task expr returned invalid cost %g", info.OriginModelName, rawCost)
	}

	groupRatioInfo := HandleGroupRatio(c, info)
	quotaBeforeGroup := rawCost / 1_000_000 * common.QuotaPerUnit
	quota, clamp := common.QuotaRoundChecked(quotaBeforeGroup * groupRatioInfo.GroupRatio)
	if quota < 0 {
		return types.PriceData{}, fmt.Errorf("model %s tiered task expr produced negative quota %d", info.OriginModelName, quota)
	}
	if clamp != nil && info.QuotaClamp == nil {
		info.QuotaClamp = clamp
	}

	freeModel := false
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume &&
		(groupRatioInfo.GroupRatio == 0 || rawCost == 0) {
		freeModel = true
		quota = 0
	}

	priceData := types.PriceData{
		FreeModel:         freeModel,
		UsePrice:          true,
		Quota:             quota,
		QuotaToPreConsume: quota,
		GroupRatioInfo:    groupRatioInfo,
	}
	snapshot := &billingexpr.BillingSnapshot{
		BillingMode:               billing_setting.BillingModeTieredExpr,
		ModelName:                 info.OriginModelName,
		ExprString:                exprStr,
		ExprHash:                  billingexpr.ExprHashString(exprStr),
		GroupRatio:                groupRatioInfo.GroupRatio,
		EstimatedPromptTokens:     0,
		EstimatedCompletionTokens: 0,
		EstimatedQuotaBeforeGroup: quotaBeforeGroup,
		EstimatedQuotaAfterGroup:  quota,
		EstimatedTier:             trace.MatchedTier,
		QuotaPerUnit:              common.QuotaPerUnit,
		ExprVersion:               billingexpr.ExprVersion(exprStr),
	}

	info.PriceData = priceData
	info.TieredBillingSnapshot = snapshot
	info.BillingRequestInput = &requestInput
	return priceData, nil
}

func modelPriceHelperTaskTokens(c *gin.Context, info *relaycommon.RelayInfo, exprStr string) (types.PriceData, error) {
	priceData, err := ModelPriceHelperPerCall(c, info)
	if err != nil {
		return types.PriceData{}, err
	}
	requestInput, err := buildTaskBillingRequestInput(c, info)
	if err != nil {
		return types.PriceData{}, err
	}
	rawCost, trace, err := billingexpr.RunExprWithRequest(exprStr, billingexpr.TokenParams{}, requestInput)
	if err != nil {
		return types.PriceData{}, fmt.Errorf("model %s token task expr run failed: %w", info.OriginModelName, err)
	}
	if math.IsNaN(rawCost) || math.IsInf(rawCost, 0) || rawCost < 0 {
		return types.PriceData{}, fmt.Errorf("model %s token task expr returned invalid cost %g", info.OriginModelName, rawCost)
	}

	quotaBeforeGroup := 0.0
	if priceData.GroupRatioInfo.GroupRatio > 0 {
		quotaBeforeGroup = float64(priceData.Quota) / priceData.GroupRatioInfo.GroupRatio
	}
	snapshot := &billingexpr.BillingSnapshot{
		BillingMode:               billing_setting.BillingModeTieredExpr,
		ModelName:                 info.OriginModelName,
		ExprString:                exprStr,
		ExprHash:                  billingexpr.ExprHashString(exprStr),
		GroupRatio:                priceData.GroupRatioInfo.GroupRatio,
		EstimatedPromptTokens:     0,
		EstimatedCompletionTokens: 0,
		EstimatedQuotaBeforeGroup: quotaBeforeGroup,
		EstimatedQuotaAfterGroup:  priceData.Quota,
		EstimatedTier:             trace.MatchedTier,
		QuotaPerUnit:              common.QuotaPerUnit,
		ExprVersion:               billingexpr.ExprVersion(exprStr),
		TaskTokenBilling:          true,
	}

	info.PriceData = priceData
	info.TieredBillingSnapshot = snapshot
	info.BillingRequestInput = &requestInput
	return priceData, nil
}

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

// ModelPriceHelperTask keeps legacy Task/MJ per-call pricing isolated while
// allowing request-aware tiered expressions to opt in through per_request().
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
	if !billingexpr.UsedVars(exprStr)["per_request"] {
		return ModelPriceHelperPerCall(c, info)
	}
	if info.TaskRelayInfo != nil && info.Action == constant.TaskActionRemix {
		return types.PriceData{}, fmt.Errorf("remix is not supported for expression-billed task models")
	}

	return modelPriceHelperTaskTiered(c, info, exprStr)
}

func modelPriceHelperTaskTiered(c *gin.Context, info *relaycommon.RelayInfo, exprStr string) (types.PriceData, error) {
	taskRequest, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return types.PriceData{}, fmt.Errorf("resolve normalized task billing request: %w", err)
	}

	requestInput, err := BuildBillingExprRequestInputFromRequest(struct {
		Model      string `json:"model"`
		Resolution string `json:"resolution,omitempty"`
		Duration   int    `json:"duration,omitempty"`
	}{
		Model:      info.OriginModelName,
		Resolution: taskRequest.Resolution,
		Duration:   taskRequest.Duration,
	}, info.RequestHeaders)
	if err != nil {
		return types.PriceData{}, fmt.Errorf("build normalized task billing request: %w", err)
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

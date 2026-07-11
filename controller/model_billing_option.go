package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/gin-gonic/gin"
)

type ModelBillingOptionsUpdateRequest struct {
	BillingMode map[string]string `json:"billing_mode"`
	BillingExpr map[string]string `json:"billing_expr"`
	Options     map[string]string `json:"options,omitempty"`
}

var atomicModelPricingOptionKeys = map[string]struct{}{
	"ModelPrice":           {},
	"ModelRatio":           {},
	"CompletionRatio":      {},
	"CacheRatio":           {},
	"CreateCacheRatio":     {},
	"ImageRatio":           {},
	"AudioRatio":           {},
	"AudioCompletionRatio": {},
	"ExposeRatioEnabled":   {},
}

func UpdateModelBillingOptions(c *gin.Context) {
	var request ModelBillingOptionsUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	if request.BillingMode == nil {
		request.BillingMode = make(map[string]string)
	}
	if request.BillingExpr == nil {
		request.BillingExpr = make(map[string]string)
	}
	if request.Options == nil {
		request.Options = make(map[string]string)
	}
	if err := billing_setting.ValidateModelBillingConfig(request.BillingMode, request.BillingExpr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	for key, value := range request.Options {
		if _, ok := atomicModelPricingOptionKeys[key]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("unsupported model pricing option %s", key)})
			return
		}
		if key == "ExposeRatioEnabled" {
			if _, err := strconv.ParseBool(value); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ExposeRatioEnabled must be a boolean"})
				return
			}
			continue
		}
		var parsed map[string]float64
		if err := common.UnmarshalJsonStr(value, &parsed); err != nil || parsed == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("%s must be a JSON object with numeric values", key)})
			return
		}
	}
	modeJSON, err := common.Marshal(request.BillingMode)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	exprJSON, err := common.Marshal(request.BillingExpr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	optionValues := make(map[string]string, len(request.Options)+2)
	for key, value := range request.Options {
		optionValues[key] = value
	}
	optionValues[billing_setting.BillingModeOptionKey] = string(modeJSON)
	optionValues[billing_setting.BillingExprOptionKey] = string(exprJSON)
	if err := model.UpdateOptionsBulk(optionValues); err != nil {
		common.ApiError(c, err)
		return
	}
	billingexpr.InvalidateCache()
	updatedKeys := make([]string, 0, len(request.Options)+2)
	updatedKeys = append(updatedKeys, billing_setting.BillingModeOptionKey, billing_setting.BillingExprOptionKey)
	for key := range request.Options {
		updatedKeys = append(updatedKeys, key)
	}
	sort.Strings(updatedKeys)
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": strings.Join(updatedKeys, ","),
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

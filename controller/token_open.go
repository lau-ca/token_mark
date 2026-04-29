package controller

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// convertRemainAmountToQuota 将 remain_amount 转换为 remain_quota
// 规则：根据 quota_display_type 确定换算方式
// - TOKENS: remain_quota = remain_amount
// - USD: remain_quota = remain_amount × QuotaPerUnit
// - CNY/CUSTOM: remain_quota = remain_amount / rate × QuotaPerUnit
func convertRemainAmountToQuota(req *tokenOpenRequest) {
	if req.RemainAmount <= 0 || req.UnlimitedQuota {
		return
	}

	quotaDisplayType := operation_setting.GetQuotaDisplayType()
	rate := operation_setting.USDExchangeRate

	var usdAmount float64
	switch quotaDisplayType {
	case operation_setting.QuotaDisplayTypeTokens:
		req.RemainQuota = int(req.RemainAmount)
		return
	case operation_setting.QuotaDisplayTypeUSD:
		usdAmount = req.RemainAmount
	case operation_setting.QuotaDisplayTypeCNY, operation_setting.QuotaDisplayTypeCustom:
		if rate > 0 {
			usdAmount = req.RemainAmount / rate
		} else {
			usdAmount = req.RemainAmount
		}
	default:
		usdAmount = req.RemainAmount
	}

	req.RemainQuota = int(usdAmount * common.QuotaPerUnit)
	common.SysLog(fmt.Sprintf("convertRemainAmountToQuota: remain_amount=%.6f, type=%s, rate=%.4f, QuotaPerUnit=%.1f, remain_quota=%d",
		req.RemainAmount, quotaDisplayType, rate, common.QuotaPerUnit, req.RemainQuota))
}

// tokenOpenRequest 开放接口创建令牌的请求结构，支持 remain_amount
type tokenOpenRequest struct {
	Name               string  `json:"name"`
	RemainQuota       int     `json:"remain_quota"`
	RemainAmount      float64 `json:"remain_amount"`
	UnlimitedQuota    bool    `json:"unlimited_quota"`
	ExpiredTime       int64   `json:"expired_time"`
	ModelLimitsEnabled bool   `json:"model_limits_enabled"`
	ModelLimits       string  `json:"model_limits"`
	AllowIps          *string `json:"allow_ips"`
	Group             string  `json:"group"`
	CrossGroupRetry   bool    `json:"cross_group_retry"`
}

// validateTokenRequest 验证令牌请求的通用逻辑
func validateTokenRequest(c *gin.Context, token *model.Token) bool {
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return false
	}
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return false
		}
		maxQuotaValue := int((1000000000 * common.QuotaPerUnit))
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return false
		}
	}
	return true
}

// createToken 通用令牌创建逻辑
func createToken(c *gin.Context, userId int, token *model.Token) {
	maxTokens := operation_setting.GetMaxUserTokens()
	count, err := model.CountUserTokens(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= maxTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("已达到最大令牌数量限制 (%d)", maxTokens),
		})
		return
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		common.SysLog("failed to generate token key: " + err.Error())
		return
	}
	token.Key = key
	token.UserId = userId
	token.CreatedTime = common.GetTimestamp()
	token.AccessedTime = common.GetTimestamp()
	if err := token.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"key":     token.Key,
	})
}

// tokenAmountOpenRequest 开放接口修改令牌金额的请求结构
type tokenAmountOpenRequest struct {
	Key          string  `json:"key" binding:"required"`
	RemainAmount float64 `json:"remain_amount" binding:"required"`
}

// UpdateTokenAmountOpen 开放接口：根据 key 修改令牌金额
// POST /api/token/open/amount
// 认证由 HMACAuth 中间件处理
func UpdateTokenAmountOpen(c *gin.Context) {
	// 1. 解析请求体
	var req tokenAmountOpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	if req.RemainAmount < 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "remain_amount cannot be negative"})
		return
	}

	// 3. 根据 key 查找令牌
	token, err := model.GetTokenByKey(req.Key, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "token not found"})
		return
	}

	common.SysLog(fmt.Sprintf("UpdateTokenAmountOpen: key=%s, remain_amount=%.6f", req.Key, req.RemainAmount))

	// 4. 将 remain_amount 转换为 remain_quota
	quotaDisplayType := operation_setting.GetQuotaDisplayType()
	rate := operation_setting.USDExchangeRate

	var usdAmount float64
	switch quotaDisplayType {
	case operation_setting.QuotaDisplayTypeTokens:
		token.RemainQuota = int(req.RemainAmount)
	case operation_setting.QuotaDisplayTypeUSD:
		usdAmount = req.RemainAmount
		token.RemainQuota = int(usdAmount * common.QuotaPerUnit)
	case operation_setting.QuotaDisplayTypeCNY, operation_setting.QuotaDisplayTypeCustom:
		if rate > 0 {
			usdAmount = req.RemainAmount / rate
		} else {
			usdAmount = req.RemainAmount
		}
		token.RemainQuota = int(usdAmount * common.QuotaPerUnit)
	default:
		usdAmount = req.RemainAmount
		token.RemainQuota = int(usdAmount * common.QuotaPerUnit)
	}

	common.SysLog(fmt.Sprintf("UpdateTokenAmountOpen: key=%s, remain_amount=%.6f, type=%s, rate=%.4f, QuotaPerUnit=%.1f, remain_quota=%d",
		req.Key, req.RemainAmount, quotaDisplayType, rate, common.QuotaPerUnit, token.RemainQuota))

	// 5. 如果令牌已耗尽且设置了新金额，则恢复为启用状态
	if token.Status == common.TokenStatusExhausted && token.RemainQuota > 0 {
		token.Status = common.TokenStatusEnabled
	}

	// 6. 更新令牌
	if err := token.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "",
		"key":          token.Key,
		"remain_quota": token.RemainQuota,
	})
}

// CreateTokenOpen 开放接口：创建新的 token（归属第一个用户）
// POST /api/token/open
// 认证由 HMACAuth 中间件处理
func CreateTokenOpen(c *gin.Context) {
	// 1. 解析请求体
	var req tokenOpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	common.SysLog(fmt.Sprintf("CreateTokenOpen: remain_amount=%.6f, unlimited_quota=%v, remain_quota=%d",
		req.RemainAmount, req.UnlimitedQuota, req.RemainQuota))

	// 3. 将 remain_amount 转换为 remain_quota
	convertRemainAmountToQuota(&req)
	common.SysLog(fmt.Sprintf("After convert: remain_quota=%d", req.RemainQuota))

	// 4. 构建 Token 对象
	token := &model.Token{
		Name:               req.Name,
		ExpiredTime:        req.ExpiredTime,
		RemainQuota:        req.RemainQuota,
		UnlimitedQuota:     req.UnlimitedQuota,
		ModelLimitsEnabled: req.ModelLimitsEnabled,
		ModelLimits:        req.ModelLimits,
		AllowIps:          req.AllowIps,
		Group:              req.Group,
		CrossGroupRetry:    req.CrossGroupRetry,
	}
	common.SysLog(fmt.Sprintf("Creating token with: remain_quota=%d, remain_amount=%.6f, QuotaPerUnit=%.1f",
		token.RemainQuota, req.RemainAmount, common.QuotaPerUnit))

	// 5. 验证并创建
	if !validateTokenRequest(c, token) {
		return
	}
	createToken(c, 1, token)
}

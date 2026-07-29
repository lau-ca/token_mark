package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	infiniTopUpTradePrefix        = "INF_TOPUP-"
	infiniSubscriptionTradePrefix = "INF_SUB-"
)

type InfiniPayRequest struct {
	Amount int64 `json:"amount"`
}

func RequestInfiniAmount(c *gin.Context) {
	var req InfiniPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount < int64(setting.InfiniMinTopUp) {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", setting.InfiniMinTopUp))
		return
	}
	group, err := model.GetUserGroup(c.GetInt("id"), true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	payMoney := getInfiniPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": decimal.NewFromFloat(payMoney).StringFixed(2)})
}

func RequestInfiniPay(c *gin.Context) {
	if !isInfiniTopUpEnabled() {
		common.ApiErrorMsg(c, "Infini 配置不完整")
		return
	}
	var req InfiniPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount < int64(setting.InfiniMinTopUp) {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", setting.InfiniMinTopUp))
		return
	}

	userID := c.GetInt("id")
	user, err := model.GetUserById(userID, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	payMoney := getInfiniPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}
	payMethods, err := getInfiniPayMethods()
	if err != nil {
		common.ApiErrorMsg(c, "Infini 支付方式配置无效")
		return
	}

	tradeNo := fmt.Sprintf("%s%d-%d-%s", infiniTopUpTradePrefix, userID, time.Now().UnixMilli(), common.GetRandomString(6))
	topUp := &model.TopUp{
		UserId:          userID,
		Amount:          normalizeInfiniTopUpAmount(req.Amount),
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodInfini,
		PaymentProvider: model.PaymentProviderInfini,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini 创建充值订单失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	result, err := newInfiniClient().CreateOrder(c.Request.Context(), &service.InfiniCreateOrderRequest{
		Amount:           decimal.NewFromFloat(payMoney).StringFixed(2),
		RequestID:        uuid.NewString(),
		ClientReference:  tradeNo,
		OrderDescription: fmt.Sprintf("Top-up %d", req.Amount),
		SuccessURL:       paymentReturnPath("/console/topup?pay=success"),
		FailureURL:       paymentReturnPath("/console/topup?pay=fail"),
		PayMethods:       payMethods,
		Email:            strings.TrimSpace(user.Email),
		Currency:         normalizeInfiniCurrency(setting.InfiniCurrency),
	})
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderInfini, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini 拉起充值失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Infini 充值订单创建成功 user_id=%d trade_no=%s infini_order_id=%s money=%.2f", userID, tradeNo, result.OrderID, payMoney))
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{
		"checkout_url": result.CheckoutURL,
		"order_id":     result.OrderID,
		"trade_no":     tradeNo,
	}})
}

func InfiniWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid body")
		return
	}
	timestamp := c.GetHeader("X-Webhook-Timestamp")
	eventID := c.GetHeader("X-Webhook-Event-Id")
	signature := c.GetHeader("X-Webhook-Signature")
	if err := service.VerifyInfiniWebhook(setting.InfiniWebhookSecret, timestamp, eventID, signature, body, time.Now()); err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Infini webhook 验签失败 event_id=%q client_ip=%s error=%q", eventID, c.ClientIP(), err.Error()))
		c.String(http.StatusUnauthorized, "invalid signature")
		return
	}

	var event service.InfiniWebhookPayload
	if err := common.Unmarshal(body, &event); err != nil || strings.TrimSpace(event.ClientReference) == "" {
		c.String(http.StatusBadRequest, "invalid payload")
		return
	}
	if event.Event != "order.completed" || event.Status != "paid" {
		if event.Event == "order.expired" {
			expireInfiniOrder(event.ClientReference)
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("Infini webhook 状态已记录 event_id=%s event=%s status=%s trade_no=%s", eventID, event.Event, event.Status, event.ClientReference))
		c.String(http.StatusOK, "OK")
		return
	}

	LockOrder(event.ClientReference)
	defer UnlockOrder(event.ClientReference)
	if strings.HasPrefix(event.ClientReference, infiniSubscriptionTradePrefix) {
		err = model.CompleteSubscriptionOrder(event.ClientReference, string(body), model.PaymentProviderInfini, model.PaymentMethodInfini)
	} else if strings.HasPrefix(event.ClientReference, infiniTopUpTradePrefix) {
		err = model.RechargeInfini(event.ClientReference, c.ClientIP())
	} else {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Infini webhook 未识别本地订单 event_id=%s trade_no=%s", eventID, event.ClientReference))
		c.String(http.StatusOK, "OK")
		return
	}
	if err != nil {
		if errors.Is(err, model.ErrPaymentMethodMismatch) || errors.Is(err, model.ErrTopUpNotFound) || errors.Is(err, model.ErrSubscriptionOrderNotFound) {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Infini webhook 订单拒绝 event_id=%s trade_no=%s error=%q", eventID, event.ClientReference, err.Error()))
			c.String(http.StatusOK, "OK")
			return
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini webhook 结算失败 event_id=%s trade_no=%s error=%q", eventID, event.ClientReference, err.Error()))
		c.String(http.StatusInternalServerError, "retry")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Infini webhook 结算成功 event_id=%s trade_no=%s order_id=%s", eventID, event.ClientReference, event.OrderID))
	c.String(http.StatusOK, "OK")
}

func newInfiniClient() *service.InfiniClient {
	return &service.InfiniClient{
		BaseURL:   service.InfiniBaseURL(setting.InfiniSandbox),
		KeyID:     setting.InfiniKeyID,
		SecretKey: setting.InfiniSecretKey,
	}
}

func getInfiniPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount = dAmount.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	groupRatio := common.GetTopupGroupRatio(group)
	if groupRatio == 0 {
		groupRatio = 1
	}
	discount := 1.0
	if value, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok && value > 0 {
		discount = value
	}
	return dAmount.
		Mul(decimal.NewFromFloat(setting.InfiniUnitPrice)).
		Mul(decimal.NewFromFloat(groupRatio)).
		Mul(decimal.NewFromFloat(discount)).InexactFloat64()
}

func normalizeInfiniTopUpAmount(amount int64) int64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return amount
	}
	normalized := decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	if normalized < 1 {
		return 1
	}
	return normalized
}

func getInfiniPayMethods() ([]int, error) {
	var methods []int
	if err := common.UnmarshalJsonStr(setting.InfiniPayMethods, &methods); err != nil || len(methods) == 0 {
		return nil, errors.New("Infini pay methods are invalid")
	}
	for _, method := range methods {
		switch method {
		case 1, 2, 3, 5, 6:
		default:
			return nil, errors.New("Infini pay method is unsupported")
		}
	}
	return methods, nil
}

func normalizeInfiniCurrency(currency string) string {
	value := strings.ToUpper(strings.TrimSpace(currency))
	switch value {
	case "USD", "EUR", "KWR", "KRW", "GBP", "SGD", "JPY", "AUD", "HKD":
		return value
	default:
		return "USD"
	}
}

func expireInfiniOrder(tradeNo string) {
	if strings.HasPrefix(tradeNo, infiniSubscriptionTradePrefix) {
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentProviderInfini)
		return
	}
	if strings.HasPrefix(tradeNo, infiniTopUpTradePrefix) {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderInfini, common.TopUpStatusExpired)
	}
}

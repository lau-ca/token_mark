package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type SubscriptionInfiniPayRequest struct {
	PlanID int `json:"plan_id"`
}

func SubscriptionRequestInfiniPay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	if !isInfiniTopUpEnabled() {
		common.ApiErrorMsg(c, "Infini 配置不完整")
		return
	}
	var req SubscriptionInfiniPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	plan, err := model.GetSubscriptionPlanById(req.PlanID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}

	userID := c.GetInt("id")
	user, err := model.GetUserById(userID, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userID, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorMsg(c, "已达到该套餐购买上限")
			return
		}
	}
	payMethods, err := getInfiniPayMethods()
	if err != nil {
		common.ApiErrorMsg(c, "Infini 支付方式配置无效")
		return
	}

	tradeNo := fmt.Sprintf("%s%d-%d-%s", infiniSubscriptionTradePrefix, userID, time.Now().UnixMilli(), common.GetRandomString(6))
	order := &model.SubscriptionOrder{
		UserId:          userID,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodInfini,
		PaymentProvider: model.PaymentProviderInfini,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini 创建订阅订单失败 user_id=%d plan_id=%d trade_no=%s error=%q", userID, plan.Id, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	result, err := newInfiniClient().CreateOrder(c.Request.Context(), &service.InfiniCreateOrderRequest{
		Amount:           decimal.NewFromFloat(plan.PriceAmount).StringFixed(2),
		RequestID:        uuid.NewString(),
		ClientReference:  tradeNo,
		OrderDescription: "Subscription: " + plan.Title,
		SuccessURL:       paymentReturnPath("/console/topup?pay=success"),
		FailureURL:       paymentReturnPath("/console/topup?pay=fail"),
		PayMethods:       payMethods,
		Email:            strings.TrimSpace(user.Email),
		Currency:         normalizeInfiniCurrency(plan.Currency),
	})
	if err != nil {
		order.Status = common.TopUpStatusFailed
		_ = order.Update()
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini 拉起订阅支付失败 user_id=%d plan_id=%d trade_no=%s error=%q", userID, plan.Id, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Infini 订阅订单创建成功 user_id=%d plan_id=%d trade_no=%s infini_order_id=%s money=%.2f", userID, plan.Id, tradeNo, result.OrderID, plan.PriceAmount))
	common.ApiSuccess(c, gin.H{
		"checkout_url": result.CheckoutURL,
		"order_id":     result.OrderID,
		"trade_no":     tradeNo,
	})
}

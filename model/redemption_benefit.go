package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
)

type RedemptionRedeemResult struct {
	BenefitType              string `json:"benefit_type"`
	Quota                    int    `json:"quota,omitempty"`
	SubscriptionId           int    `json:"subscription_id,omitempty"`
	SubscriptionPlanId       int    `json:"subscription_plan_id,omitempty"`
	SubscriptionPlanTitle    string `json:"subscription_plan_title,omitempty"`
	subscriptionUpgradeGroup string
}

func (redemption *Redemption) BeforeCreate(tx *gorm.DB) error {
	redemption.NormalizeBenefitFields()
	return nil
}

func (redemption *Redemption) BeforeUpdate(tx *gorm.DB) error {
	redemption.NormalizeBenefitFields()
	return nil
}

func (redemption *Redemption) AfterFind(tx *gorm.DB) error {
	redemption.NormalizeBenefitFields()
	return nil
}

func normalizeRedemptionBenefitType(benefitType string) string {
	switch strings.TrimSpace(benefitType) {
	case common.RedemptionBenefitTypeSubscription:
		return common.RedemptionBenefitTypeSubscription
	default:
		return common.RedemptionBenefitTypeQuota
	}
}

func (redemption *Redemption) NormalizeBenefitFields() {
	if redemption == nil {
		return
	}
	redemption.BenefitType = normalizeRedemptionBenefitType(redemption.BenefitType)
	if redemption.BenefitType == common.RedemptionBenefitTypeSubscription {
		redemption.Quota = 0
		return
	}
	redemption.SubscriptionPlanId = 0
	redemption.RedeemedSubscriptionId = 0
}

func (redemption *Redemption) ValidateBenefitForUpsert(tx *gorm.DB) error {
	if redemption == nil {
		return errors.New("invalid redemption")
	}
	redemption.NormalizeBenefitFields()
	if redemption.BenefitType != common.RedemptionBenefitTypeSubscription {
		return nil
	}
	if redemption.SubscriptionPlanId <= 0 {
		return errors.New("订阅套餐不能为空")
	}
	_, err := getSubscriptionPlanByIdTx(tx, redemption.SubscriptionPlanId)
	return err
}

func (redemption *Redemption) redeemBenefitTx(tx *gorm.DB, userId int) (*RedemptionRedeemResult, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if redemption == nil {
		return nil, errors.New("invalid redemption")
	}
	redemption.NormalizeBenefitFields()
	switch redemption.BenefitType {
	case common.RedemptionBenefitTypeSubscription:
		return redemption.redeemSubscriptionBenefitTx(tx, userId)
	default:
		return redemption.redeemQuotaBenefitTx(tx, userId)
	}
}

func (redemption *Redemption) redeemQuotaBenefitTx(tx *gorm.DB, userId int) (*RedemptionRedeemResult, error) {
	err := tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
	if err != nil {
		return nil, err
	}
	return &RedemptionRedeemResult{
		BenefitType: common.RedemptionBenefitTypeQuota,
		Quota:       redemption.Quota,
	}, nil
}

func (redemption *Redemption) redeemSubscriptionBenefitTx(tx *gorm.DB, userId int) (*RedemptionRedeemResult, error) {
	plan, err := getSubscriptionPlanByIdTx(tx, redemption.SubscriptionPlanId)
	if err != nil {
		return nil, err
	}
	sub, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "redemption")
	if err != nil {
		return nil, err
	}
	redemption.RedeemedSubscriptionId = sub.Id
	return &RedemptionRedeemResult{
		BenefitType:              common.RedemptionBenefitTypeSubscription,
		SubscriptionId:           sub.Id,
		SubscriptionPlanId:       plan.Id,
		SubscriptionPlanTitle:    plan.Title,
		subscriptionUpgradeGroup: strings.TrimSpace(plan.UpgradeGroup),
	}, nil
}

func (result *RedemptionRedeemResult) LegacyQuota() int {
	if result == nil || result.BenefitType != common.RedemptionBenefitTypeQuota {
		return 0
	}
	return result.Quota
}

func (result *RedemptionRedeemResult) LogMessage(redemptionId int) string {
	if result == nil {
		return ""
	}
	if result.BenefitType == common.RedemptionBenefitTypeSubscription {
		return fmt.Sprintf("通过兑换码开通订阅 %s，订阅ID %d，兑换码ID %d", result.SubscriptionPlanTitle, result.SubscriptionId, redemptionId)
	}
	return fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(result.Quota), redemptionId)
}

package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareRedemptionBenefitTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Redemption{}).Error)
	require.NoError(t, DB.Exec("DELETE FROM redemptions").Error)
	require.NoError(t, DB.Exec("DELETE FROM user_subscriptions").Error)
	require.NoError(t, DB.Exec("DELETE FROM subscription_plans").Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM logs").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM redemptions")
		DB.Exec("DELETE FROM user_subscriptions")
		DB.Exec("DELETE FROM subscription_plans")
		DB.Exec("DELETE FROM users")
		DB.Exec("DELETE FROM logs")
	})
}

func insertRedemptionBenefitUser(t *testing.T, id int, quota int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       id,
		Username: "redemption_benefit_user",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}).Error)
}

func TestRedeemQuotaBenefitKeepsWalletFlow(t *testing.T) {
	prepareRedemptionBenefitTest(t)
	insertRedemptionBenefitUser(t, 6101, 100)

	redemption := &Redemption{
		Name:        "quota code",
		Key:         "quota-benefit-code",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       250,
		BenefitType: common.RedemptionBenefitTypeQuota,
	}
	require.NoError(t, redemption.Insert())

	result, err := Redeem("quota-benefit-code", 6101)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, common.RedemptionBenefitTypeQuota, result.BenefitType)
	assert.Equal(t, 250, result.LegacyQuota())

	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", 6101).First(&user).Error)
	assert.Equal(t, 350, user.Quota)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", redemption.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, saved.Status)
	assert.Equal(t, 6101, saved.UsedUserId)
	assert.Zero(t, saved.RedeemedSubscriptionId)
}

func TestRedeemSubscriptionBenefitCreatesSubscription(t *testing.T) {
	prepareRedemptionBenefitTest(t)
	insertRedemptionBenefitUser(t, 6201, 100)

	plan := &SubscriptionPlan{
		Id:            7201,
		Title:         "Redeem Plan",
		PriceAmount:   0,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   5000,
	}
	require.NoError(t, DB.Create(plan).Error)

	redemption := &Redemption{
		Name:               "subscription code",
		Key:                "subscription-benefit-code",
		Status:             common.RedemptionCodeStatusEnabled,
		BenefitType:        common.RedemptionBenefitTypeSubscription,
		SubscriptionPlanId: plan.Id,
	}
	require.NoError(t, redemption.Insert())

	var storedQuota int
	require.NoError(t, DB.Model(&Redemption{}).Where("id = ?", redemption.Id).Select("quota").Scan(&storedQuota).Error)
	assert.Zero(t, storedQuota)

	result, err := Redeem("subscription-benefit-code", 6201)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, common.RedemptionBenefitTypeSubscription, result.BenefitType)
	assert.Equal(t, plan.Id, result.SubscriptionPlanId)
	assert.Equal(t, "Redeem Plan", result.SubscriptionPlanTitle)
	assert.Zero(t, result.LegacyQuota())

	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", 6201).First(&user).Error)
	assert.Equal(t, 100, user.Quota)

	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", result.SubscriptionId).First(&sub).Error)
	assert.Equal(t, 6201, sub.UserId)
	assert.Equal(t, plan.Id, sub.PlanId)
	assert.Equal(t, int64(5000), sub.AmountTotal)
	assert.Equal(t, "redemption", sub.Source)
	assert.Equal(t, "active", sub.Status)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", redemption.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, saved.Status)
	assert.Equal(t, result.SubscriptionId, saved.RedeemedSubscriptionId)
}

func TestRedeemSubscriptionBenefitFailureDoesNotConsumeCode(t *testing.T) {
	prepareRedemptionBenefitTest(t)
	insertRedemptionBenefitUser(t, 6301, 100)

	plan := &SubscriptionPlan{
		Id:                 7301,
		Title:              "Limited Redeem Plan",
		PriceAmount:        0,
		Currency:           "USD",
		DurationUnit:       SubscriptionDurationMonth,
		DurationValue:      1,
		Enabled:            true,
		TotalAmount:        5000,
		MaxPurchasePerUser: 1,
	}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:      6301,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   common.GetTimestamp() - 10,
		EndTime:     common.GetTimestamp() + 3600,
		Status:      "active",
		Source:      "admin",
	}).Error)

	redemption := &Redemption{
		Name:               "limited subscription code",
		Key:                "limited-subscription-benefit-code",
		Status:             common.RedemptionCodeStatusEnabled,
		BenefitType:        common.RedemptionBenefitTypeSubscription,
		SubscriptionPlanId: plan.Id,
	}
	require.NoError(t, redemption.Insert())

	result, err := Redeem("limited-subscription-benefit-code", 6301)
	require.ErrorIs(t, err, ErrRedeemFailed)
	assert.Nil(t, result)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", redemption.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, saved.Status)
	assert.Zero(t, saved.UsedUserId)
	assert.Zero(t, saved.RedeemedTime)
	assert.Zero(t, saved.RedeemedSubscriptionId)

	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", 6301, plan.Id).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

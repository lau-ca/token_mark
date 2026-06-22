package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	DistributionWithdrawalStatusPending  = "pending"
	DistributionWithdrawalStatusApproved = "approved"
	DistributionWithdrawalStatusRejected = "rejected"

	DistributionWithdrawalMethodAlipay   = "alipay"
	DistributionWithdrawalMethodPlatform = "platform"
)

type DistributionWithdrawal struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id" gorm:"index"`
	Username      string `json:"username" gorm:"type:varchar(64);index"`
	Amount        int64  `json:"amount"`
	Method        string `json:"method" gorm:"type:varchar(32);index"`
	Account       string `json:"account" gorm:"type:varchar(128)"`
	RealName      string `json:"real_name" gorm:"type:varchar(64)"`
	Status        string `json:"status" gorm:"type:varchar(32);index"`
	Remark        string `json:"remark" gorm:"type:varchar(255)"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
	HandledAt     int64  `json:"handled_at" gorm:"default:0"`
	HandledBy     int    `json:"handled_by" gorm:"default:0"`
	HandledByName string `json:"handled_by_name" gorm:"type:varchar(64)"`
}

type DistributionInviteUser struct {
	Id            int    `json:"id"`
	Username      string `json:"username"`
	Quota         int    `json:"quota"`
	UsedQuota     int    `json:"used_quota"`
	TotalQuota    int64  `json:"total_quota"`
	CreatedAt     int64  `json:"created_at"`
	LastLoginAt   int64  `json:"last_login_at"`
	RechargeQuota int64  `json:"recharge_quota"`
}

type DistributionSummary struct {
	InviteCount     int64 `json:"invite_count"`
	InviteQuota     int64 `json:"invite_quota"`
	TotalAmount     int64 `json:"total_amount"`
	ApprovedAmount  int64 `json:"approved_amount"`
	PendingAmount   int64 `json:"pending_amount"`
	AvailableAmount int64 `json:"available_amount"`
}

type DistributionAdminSummary struct {
	PendingCount   int64 `json:"pending_count"`
	PendingAmount  int64 `json:"pending_amount"`
	ApprovedAmount int64 `json:"approved_amount"`
	RejectedAmount int64 `json:"rejected_amount"`
}

var (
	ErrDistributionWithdrawalAmountInvalid = errors.New("提现额度无效")
	ErrDistributionWithdrawalMethodInvalid = errors.New("提现方式无效")
	ErrDistributionWithdrawalInsufficient  = errors.New("可提现额度不足")
	ErrDistributionWithdrawalStatusInvalid = errors.New("提现申请状态无效")
)

func normalizeDistributionWithdrawalMethod(method string) string {
	switch strings.TrimSpace(method) {
	case DistributionWithdrawalMethodAlipay:
		return DistributionWithdrawalMethodAlipay
	case DistributionWithdrawalMethodPlatform:
		return DistributionWithdrawalMethodPlatform
	default:
		return ""
	}
}

func calculateTopUpQuota(topUp TopUp) int64 {
	if topUp.Status != common.TopUpStatusSuccess {
		return 0
	}
	switch topUp.PaymentProvider {
	case PaymentProviderCreem:
		return topUp.Amount
	case PaymentProviderEpay:
		return decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	case PaymentProviderWaffo:
		return decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	case PaymentProviderWaffoPancake:
		return decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	default:
		return decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	}
}

func getUserRechargeQuotaMap(tx *gorm.DB, userIds []int) (map[int]int64, error) {
	result := make(map[int]int64, len(userIds))
	if len(userIds) == 0 {
		return result, nil
	}

	var topUps []TopUp
	err := tx.Where("user_id IN ? AND status = ? AND amount > 0", userIds, common.TopUpStatusSuccess).Find(&topUps).Error
	if err != nil {
		return nil, err
	}

	for _, topUp := range topUps {
		result[topUp.UserId] += calculateTopUpQuota(topUp)
	}
	return result, nil
}

func getDistributionInviteUsers(tx *gorm.DB, userId int) ([]DistributionInviteUser, error) {
	var users []User
	err := tx.Where("inviter_id = ?", userId).Order("id desc").Omit("password").Find(&users).Error
	if err != nil {
		return nil, err
	}

	userIds := make([]int, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}

	rechargeQuotaMap, err := getUserRechargeQuotaMap(tx, userIds)
	if err != nil {
		return nil, err
	}

	result := make([]DistributionInviteUser, 0, len(users))
	for _, user := range users {
		rechargeQuota := rechargeQuotaMap[user.Id]
		result = append(result, DistributionInviteUser{
			Id:            user.Id,
			Username:      user.Username,
			Quota:         user.Quota,
			UsedQuota:     user.UsedQuota,
			TotalQuota:    int64(user.Quota + user.UsedQuota),
			CreatedAt:     user.CreatedAt,
			LastLoginAt:   user.LastLoginAt,
			RechargeQuota: rechargeQuota,
		})
	}

	return result, nil
}

func GetDistributionInviteUsers(userId int) ([]DistributionInviteUser, error) {
	return getDistributionInviteUsers(DB, userId)
}

func getDistributionSummary(tx *gorm.DB, userId int) (DistributionSummary, error) {
	inviteUsers, err := getDistributionInviteUsers(tx, userId)
	if err != nil {
		return DistributionSummary{}, err
	}

	var inviteQuota int64
	for _, user := range inviteUsers {
		inviteQuota += user.RechargeQuota
	}

	summary := DistributionSummary{
		InviteCount: int64(len(inviteUsers)),
		InviteQuota: inviteQuota,
		TotalAmount: inviteQuota / 10,
	}

	var withdrawals []DistributionWithdrawal
	err = tx.Where("user_id = ? AND status IN ?", userId, []string{
		DistributionWithdrawalStatusPending,
		DistributionWithdrawalStatusApproved,
	}).Find(&withdrawals).Error
	if err != nil {
		return DistributionSummary{}, err
	}

	for _, withdrawal := range withdrawals {
		if withdrawal.Status == DistributionWithdrawalStatusPending {
			summary.PendingAmount += withdrawal.Amount
		}
		if withdrawal.Status == DistributionWithdrawalStatusApproved {
			summary.ApprovedAmount += withdrawal.Amount
		}
	}

	summary.AvailableAmount = summary.TotalAmount - summary.ApprovedAmount - summary.PendingAmount
	if summary.AvailableAmount < 0 {
		summary.AvailableAmount = 0
	}
	return summary, nil
}

func GetDistributionSummary(userId int) (DistributionSummary, error) {
	return getDistributionSummary(DB, userId)
}

func CreateDistributionWithdrawal(userId int, amount int64, method string, account string, realName string) error {
	method = normalizeDistributionWithdrawalMethod(method)
	if method == "" {
		return ErrDistributionWithdrawalMethodInvalid
	}
	if amount <= 0 {
		return ErrDistributionWithdrawalAmountInvalid
	}
	if method == DistributionWithdrawalMethodAlipay && (strings.TrimSpace(account) == "" || strings.TrimSpace(realName) == "") {
		return ErrDistributionWithdrawalMethodInvalid
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, userId).Error; err != nil {
			return err
		}

		summary, err := getDistributionSummary(tx, userId)
		if err != nil {
			return err
		}
		if amount > summary.AvailableAmount {
			return ErrDistributionWithdrawalInsufficient
		}

		withdrawal := DistributionWithdrawal{
			UserId:   userId,
			Username: user.Username,
			Amount:   amount,
			Method:   method,
			Account:  strings.TrimSpace(account),
			RealName: strings.TrimSpace(realName),
			Status:   DistributionWithdrawalStatusPending,
		}
		return tx.Create(&withdrawal).Error
	})
}

func GetDistributionWithdrawals(userId int, pageInfo *common.PageInfo) ([]DistributionWithdrawal, int64, error) {
	var withdrawals []DistributionWithdrawal
	var total int64
	query := DB.Model(&DistributionWithdrawal{}).Where("user_id = ?", userId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&withdrawals).Error
	return withdrawals, total, err
}

func GetAllDistributionWithdrawals(pageInfo *common.PageInfo, status string, method string, username string) ([]DistributionWithdrawal, int64, error) {
	var withdrawals []DistributionWithdrawal
	var total int64
	query := DB.Model(&DistributionWithdrawal{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&withdrawals).Error
	return withdrawals, total, err
}

func GetDistributionAdminSummary() (DistributionAdminSummary, error) {
	var withdrawals []DistributionWithdrawal
	err := DB.Find(&withdrawals).Error
	if err != nil {
		return DistributionAdminSummary{}, err
	}

	var summary DistributionAdminSummary
	for _, withdrawal := range withdrawals {
		switch withdrawal.Status {
		case DistributionWithdrawalStatusPending:
			summary.PendingCount++
			summary.PendingAmount += withdrawal.Amount
		case DistributionWithdrawalStatusApproved:
			summary.ApprovedAmount += withdrawal.Amount
		case DistributionWithdrawalStatusRejected:
			summary.RejectedAmount += withdrawal.Amount
		}
	}
	return summary, nil
}

func HandleDistributionWithdrawal(id int, approved bool, remark string, adminId int, adminName string) (*DistributionWithdrawal, error) {
	var handledWithdrawal DistributionWithdrawal
	err := DB.Transaction(func(tx *gorm.DB) error {
		var withdrawal DistributionWithdrawal
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&withdrawal, id).Error; err != nil {
			return err
		}
		if withdrawal.Status != DistributionWithdrawalStatusPending {
			return ErrDistributionWithdrawalStatusInvalid
		}

		withdrawal.Remark = strings.TrimSpace(remark)
		withdrawal.HandledAt = common.GetTimestamp()
		withdrawal.HandledBy = adminId
		withdrawal.HandledByName = adminName

		if approved {
			withdrawal.Status = DistributionWithdrawalStatusApproved
			if withdrawal.Method == DistributionWithdrawalMethodPlatform {
				if err := tx.Model(&User{}).Where("id = ?", withdrawal.UserId).Update("quota", gorm.Expr("quota + ?", withdrawal.Amount)).Error; err != nil {
					return err
				}
			}
		} else {
			withdrawal.Status = DistributionWithdrawalStatusRejected
		}

		if err := tx.Save(&withdrawal).Error; err != nil {
			return err
		}
		handledWithdrawal = withdrawal
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &handledWithdrawal, nil
}

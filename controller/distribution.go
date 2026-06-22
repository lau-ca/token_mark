package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type createDistributionWithdrawalRequest struct {
	Amount   int64  `json:"amount"`
	Method   string `json:"method"`
	Account  string `json:"account"`
	RealName string `json:"real_name"`
}

type handleDistributionWithdrawalRequest struct {
	Approved bool   `json:"approved"`
	Remark   string `json:"remark"`
}

func GetDistribution(c *gin.Context) {
	userId := c.GetInt("id")

	summary, err := model.GetDistributionSummary(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	inviteUsers, err := model.GetDistributionInviteUsers(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo := common.GetPageQuery(c)
	withdrawals, total, err := model.GetDistributionWithdrawals(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(withdrawals)

	common.ApiSuccess(c, gin.H{
		"summary":      summary,
		"invite_users": inviteUsers,
		"withdrawals":  pageInfo,
	})
}

func CreateDistributionWithdrawal(c *gin.Context) {
	userId := c.GetInt("id")
	var req createDistributionWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	err := model.CreateDistributionWithdrawal(userId, req.Amount, req.Method, req.Account, req.RealName)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrDistributionWithdrawalAmountInvalid),
			errors.Is(err, model.ErrDistributionWithdrawalMethodInvalid),
			errors.Is(err, model.ErrDistributionWithdrawalInsufficient):
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
		default:
			common.ApiError(c, err)
		}
		return
	}

	common.ApiSuccess(c, nil)
}

func GetAllDistributionWithdrawals(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	withdrawals, total, err := model.GetAllDistributionWithdrawals(
		pageInfo,
		c.Query("status"),
		c.Query("method"),
		c.Query("username"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	summary, err := model.GetDistributionAdminSummary()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(withdrawals)

	common.ApiSuccess(c, gin.H{
		"summary":     summary,
		"withdrawals": pageInfo,
	})
}

func HandleDistributionWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("无效的申请 ID"))
		return
	}

	var req handleDistributionWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	withdrawal, err := model.HandleDistributionWithdrawal(
		id,
		req.Approved,
		req.Remark,
		c.GetInt("id"),
		c.GetString("username"),
	)
	if err != nil {
		if errors.Is(err, model.ErrDistributionWithdrawalStatusInvalid) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		common.ApiError(c, err)
		return
	}

	if req.Approved && withdrawal.Method == model.DistributionWithdrawalMethodPlatform {
		_ = model.InvalidateUserCache(withdrawal.UserId)
	}

	common.ApiSuccess(c, nil)
}

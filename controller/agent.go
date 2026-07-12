package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseAgentUserID(c *gin.Context) (int, bool) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "invalid agent user id")
		return 0, false
	}
	return userID, true
}

func getAgentProfileData(userID int) (gin.H, error) {
	profile, err := model.GetAgentProfile(model.DB, userID)
	if err != nil {
		return nil, err
	}
	versions, err := model.GetAgentMarginVersions(model.DB, userID)
	if err != nil {
		return nil, err
	}
	var current *model.AgentMarginVersion
	if len(versions) > 0 {
		current = &versions[len(versions)-1]
	}
	username, _ := model.GetUsernameById(userID, true)
	return gin.H{"profile": profile, "current_version": current, "username": username}, nil
}

func GetAgentProfiles(c *gin.Context) {
	includeDisabled := c.Query("include_disabled") == "true"
	rows, err := model.ListAgentProfiles(model.DB, c.Query("keyword"), includeDisabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func GetAgentProfileAdmin(c *gin.Context) {
	userID, ok := parseAgentUserID(c)
	if !ok {
		return
	}
	data, err := getAgentProfileData(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func GetAgentProfileSelf(c *gin.Context) {
	userID := c.GetInt("id")
	profile, err := model.GetAgentProfile(model.DB, userID)
	if err != nil || !profile.Enabled {
		common.ApiErrorMsg(c, "agent is not enabled")
		return
	}
	common.ApiSuccess(c, gin.H{"profile": gin.H{"user_id": profile.UserID, "enabled": profile.Enabled}, "username": c.GetString("username")})
}

func UpdateAgentProfile(c *gin.Context) {
	userID, ok := parseAgentUserID(c)
	if !ok {
		return
	}
	target, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTargetRole(c.GetInt("role"), target.Role) {
		common.ApiErrorMsg(c, "no permission to manage this agent")
		return
	}
	var request dto.AgentConfigRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Enabled == nil || request.PlatformRetentionRate == nil {
		common.ApiErrorMsg(c, "invalid agent configuration")
		return
	}
	remark := ""
	if request.Remark != nil {
		remark = *request.Remark
	}
	margins := make([]model.AgentGroupMarginInput, 0, len(request.GroupMargins))
	for _, margin := range request.GroupMargins {
		margins = append(margins, model.AgentGroupMarginInput{Group: margin.Group, GrossMarginRate: margin.GrossMarginRate})
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		return model.SaveAgentConfigTx(tx, userID, *request.Enabled, *request.PlatformRetentionRate, remark, margins, c.GetInt("id"), common.GetTimestamp())
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data, err := getAgentProfileData(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func parseAgentStatsQuery(c *gin.Context) (int64, int64, int, int, bool) {
	startTime, err := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	if err != nil || startTime <= 0 {
		common.ApiErrorMsg(c, "invalid start_timestamp")
		return 0, 0, 0, 0, false
	}
	endTime, err := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if err != nil || endTime < startTime {
		common.ApiErrorMsg(c, "invalid end_timestamp")
		return 0, 0, 0, 0, false
	}
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 || pageSize < 1 || pageSize > 100 {
		common.ApiErrorMsg(c, "invalid pagination")
		return 0, 0, 0, 0, false
	}
	return startTime, endTime, page, pageSize, true
}

func writeAgentStats(c *gin.Context, agentUserID int, includePlatform bool) {
	startTime, endTime, page, pageSize, ok := parseAgentStatsQuery(c)
	if !ok {
		return
	}
	result, err := service.CalculateAgentStats(model.DB, agentUserID, startTime, endTime, c.Query("keyword"), c.Query("group"), page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !includePlatform {
		result.Summary.PlatformRetainedQuota = 0
		for index := range result.Details {
			result.Details[index].PlatformRetainedQuota = 0
		}
	}
	common.ApiSuccess(c, result)
}

func GetAgentStatsAdmin(c *gin.Context) {
	userID, ok := parseAgentUserID(c)
	if ok {
		writeAgentStats(c, userID, true)
	}
}

func GetAgentStatsSelf(c *gin.Context) {
	userID := c.GetInt("id")
	if !model.IsEnabledAgent(model.DB, userID) {
		common.ApiErrorMsg(c, "agent is not enabled")
		return
	}
	writeAgentStats(c, userID, false)
}

func GetAgentSettlementsAdmin(c *gin.Context) {
	userID, ok := parseAgentUserID(c)
	if !ok {
		return
	}
	rows, err := model.ListAgentSettlements(model.DB, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func GetAgentSettlementsSelf(c *gin.Context) {
	userID := c.GetInt("id")
	if !model.IsEnabledAgent(model.DB, userID) {
		common.ApiErrorMsg(c, "agent is not enabled")
		return
	}
	rows, err := model.ListAgentSettlements(model.DB, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, gin.H{
			"id":                   row.ID,
			"agent_user_id":        row.AgentUserID,
			"period_start":         row.PeriodStart,
			"period_end":           row.PeriodEnd,
			"consumption_quota":    row.ConsumptionQuota,
			"gross_profit_quota":   row.GrossProfitQuota,
			"agent_earnings_quota": row.AgentEarningsQuota,
			"payment_reference":    row.PaymentReference,
			"confirmed_by":         row.ConfirmedBy,
			"confirmed_at":         row.ConfirmedAt,
		})
	}
	common.ApiSuccess(c, data)
}

func PreviewAgentSettlement(c *gin.Context) {
	userID, ok := parseAgentUserID(c)
	if !ok {
		return
	}
	cutoff, err := strconv.ParseInt(c.Query("cutoff"), 10, 64)
	if err != nil {
		common.ApiErrorMsg(c, "invalid settlement cutoff")
		return
	}
	preview, err := service.PreviewAgentSettlement(model.DB, userID, cutoff)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func ConfirmAgentSettlement(c *gin.Context) {
	userID, ok := parseAgentUserID(c)
	if !ok {
		return
	}
	var request dto.AgentSettlementRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Cutoff == nil {
		common.ApiErrorMsg(c, "invalid settlement request")
		return
	}
	reference := ""
	if request.PaymentReference != nil {
		reference = strings.TrimSpace(*request.PaymentReference)
	}
	settlement, err := service.ConfirmAgentSettlement(userID, *request.Cutoff, reference, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, service.ErrAgentNoUnsettledEarning) {
			common.ApiErrorMsg(c, "no unsettled agent earnings")
			return
		}
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": settlement})
}

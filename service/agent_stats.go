package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type AgentStatsSummary struct {
	CustomerCount         int `json:"customer_count"`
	ConsumptionQuota      int `json:"consumption_quota"`
	GrossProfitQuota      int `json:"gross_profit_quota"`
	PlatformRetainedQuota int `json:"platform_retained_quota,omitempty"`
	AgentEarningsQuota    int `json:"agent_earnings_quota"`
	SettledQuota          int `json:"settled_quota"`
	PendingQuota          int `json:"pending_quota"`
}

type AgentStatsDetail struct {
	CustomerUserID        int      `json:"customer_user_id"`
	Username              string   `json:"username"`
	Group                 string   `json:"group"`
	ConsumptionQuota      int      `json:"consumption_quota"`
	GrossMarginRate       *float64 `json:"gross_margin_rate"`
	GrossProfitQuota      int      `json:"gross_profit_quota"`
	PlatformRetainedQuota int      `json:"platform_retained_quota,omitempty"`
	AgentEarningsQuota    int      `json:"agent_earnings_quota"`
	Configured            bool     `json:"configured"`
	UnconfiguredQuota     int      `json:"unconfigured_quota,omitempty"`
}

type AgentStatsResult struct {
	Summary AgentStatsSummary  `json:"summary"`
	Details []AgentStatsDetail `json:"details"`
	Total   int                `json:"total"`
}

type agentDetailAccumulator struct {
	AgentStatsDetail
	allConfigured bool
}

type agentRuleAccumulator struct {
	CustomerUserID int
	Username       string
	Group          string
	Quota          int
	Margin         *model.AgentGroupMargin
}

func CalculateAgentStats(tx *gorm.DB, agentUserID int, startTime int64, endTime int64, keyword string, group string, page int, pageSize int) (*AgentStatsResult, error) {
	if agentUserID <= 0 || startTime <= 0 || endTime < startTime {
		return nil, fmt.Errorf("invalid agent statistics range")
	}
	var assignments []model.AgentCustomerAssignment
	if err := tx.Where("agent_user_id = ? AND effective_from <= ? AND (effective_to IS NULL OR effective_to >= ?)", agentUserID, endTime, startTime).
		Order("customer_user_id asc, effective_from asc").Find(&assignments).Error; err != nil {
		return nil, err
	}
	versions, err := model.GetAgentMarginVersions(tx, agentUserID)
	if err != nil {
		return nil, err
	}

	result := &AgentStatsResult{Details: make([]AgentStatsDetail, 0)}
	if len(assignments) == 0 || len(versions) == 0 {
		settled, err := sumAgentSettlements(tx, agentUserID)
		if err != nil {
			return nil, err
		}
		result.Summary.SettledQuota = settled
		return result, nil
	}

	customerIDs := make([]int, 0)
	seenCustomers := make(map[int]struct{})
	for _, assignment := range assignments {
		if _, ok := seenCustomers[assignment.CustomerUserID]; ok {
			continue
		}
		seenCustomers[assignment.CustomerUserID] = struct{}{}
		customerIDs = append(customerIDs, assignment.CustomerUserID)
	}

	logTx := model.LOG_DB
	if model.LOG_DB == model.DB {
		logTx = tx
	}
	rows, err := model.GetAgentConsumeLogRows(logTx, customerIDs, startTime, endTime)
	if err != nil {
		return nil, err
	}

	assignmentByCustomer := make(map[int][]model.AgentCustomerAssignment)
	for _, assignment := range assignments {
		assignmentByCustomer[assignment.CustomerUserID] = append(assignmentByCustomer[assignment.CustomerUserID], assignment)
	}
	versionMargins := make(map[int]map[string]model.AgentGroupMargin, len(versions))
	for _, version := range versions {
		margins := make(map[string]model.AgentGroupMargin, len(version.GroupMargins))
		for _, margin := range version.GroupMargins {
			margins[margin.Group] = margin
		}
		versionMargins[version.ID] = margins
	}

	ruleAccumulators := make(map[string]*agentRuleAccumulator)
	for _, row := range rows {
		if !agentAssignmentCovers(assignmentByCustomer[row.UserID], row.CreatedAt) {
			continue
		}
		version := effectiveAgentMarginVersion(versions, row.CreatedAt)
		versionID := 0
		var margin *model.AgentGroupMargin
		if version != nil {
			versionID = version.ID
			if configuredMargin, ok := versionMargins[version.ID][row.UseGroup]; ok && configuredMargin.PlatformRetentionRate != nil {
				marginCopy := configuredMargin
				margin = &marginCopy
			}
		}
		key := fmt.Sprintf("%d\x00%s\x00%d", row.UserID, row.UseGroup, versionID)
		segment := ruleAccumulators[key]
		if segment == nil {
			segment = &agentRuleAccumulator{
				CustomerUserID: row.UserID,
				Username:       row.Username,
				Group:          row.UseGroup,
				Margin:         margin,
			}
			ruleAccumulators[key] = segment
		}
		segment.Quota += row.Quota
	}

	accumulators := make(map[string]*agentDetailAccumulator)
	for _, segment := range ruleAccumulators {
		configured := segment.Margin != nil
		grossProfit := 0
		platformRetained := 0
		agentEarnings := 0
		if configured {
			grossProfit, _ = common.QuotaFromFloatChecked(float64(segment.Quota) * segment.Margin.GrossMarginRate)
			platformRetained, _ = common.QuotaFromFloatChecked(float64(grossProfit) * *segment.Margin.PlatformRetentionRate)
			agentEarnings = grossProfit - platformRetained
			if agentEarnings < 0 {
				agentEarnings = 0
			}
		}

		key := fmt.Sprintf("%d\x00%s", segment.CustomerUserID, segment.Group)
		accumulator := accumulators[key]
		if accumulator == nil {
			accumulator = &agentDetailAccumulator{
				AgentStatsDetail: AgentStatsDetail{CustomerUserID: segment.CustomerUserID, Username: segment.Username, Group: segment.Group},
				allConfigured:    true,
			}
			accumulators[key] = accumulator
		}
		accumulator.ConsumptionQuota += segment.Quota
		accumulator.GrossProfitQuota += grossProfit
		accumulator.PlatformRetainedQuota += platformRetained
		accumulator.AgentEarningsQuota += agentEarnings
		if !configured {
			accumulator.allConfigured = false
			accumulator.UnconfiguredQuota += segment.Quota
		}
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	activeCustomers := make(map[int]struct{})
	for _, accumulator := range accumulators {
		accumulator.Configured = accumulator.allConfigured
		if accumulator.ConsumptionQuota > 0 && accumulator.allConfigured {
			rate := float64(accumulator.GrossProfitQuota) / float64(accumulator.ConsumptionQuota)
			accumulator.GrossMarginRate = &rate
		}
		if group != "" && accumulator.Group != group {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(accumulator.Username), keyword) && !strings.Contains(fmt.Sprintf("%d", accumulator.CustomerUserID), keyword) {
			continue
		}
		result.Summary.ConsumptionQuota += accumulator.ConsumptionQuota
		result.Summary.GrossProfitQuota += accumulator.GrossProfitQuota
		result.Summary.PlatformRetainedQuota += accumulator.PlatformRetainedQuota
		result.Summary.AgentEarningsQuota += accumulator.AgentEarningsQuota
		activeCustomers[accumulator.CustomerUserID] = struct{}{}
		result.Details = append(result.Details, accumulator.AgentStatsDetail)
	}
	result.Summary.CustomerCount = len(activeCustomers)
	settled, err := sumAgentSettlements(tx, agentUserID)
	if err != nil {
		return nil, err
	}
	result.Summary.SettledQuota = settled
	result.Summary.PendingQuota = result.Summary.AgentEarningsQuota - settled
	if result.Summary.PendingQuota < 0 {
		result.Summary.PendingQuota = 0
	}

	sort.Slice(result.Details, func(i, j int) bool {
		if result.Details[i].AgentEarningsQuota == result.Details[j].AgentEarningsQuota {
			if result.Details[i].CustomerUserID == result.Details[j].CustomerUserID {
				return result.Details[i].Group < result.Details[j].Group
			}
			return result.Details[i].CustomerUserID < result.Details[j].CustomerUserID
		}
		return result.Details[i].AgentEarningsQuota > result.Details[j].AgentEarningsQuota
	})
	result.Total = len(result.Details)
	if pageSize > 0 {
		if page < 1 {
			page = 1
		}
		start := (page - 1) * pageSize
		if start >= len(result.Details) {
			result.Details = []AgentStatsDetail{}
		} else {
			end := start + pageSize
			if end > len(result.Details) {
				end = len(result.Details)
			}
			result.Details = result.Details[start:end]
		}
	}
	return result, nil
}

func agentAssignmentCovers(assignments []model.AgentCustomerAssignment, timestamp int64) bool {
	for _, assignment := range assignments {
		if timestamp < assignment.EffectiveFrom {
			continue
		}
		if assignment.EffectiveTo == nil || timestamp < *assignment.EffectiveTo {
			return true
		}
	}
	return false
}

func effectiveAgentMarginVersion(versions []model.AgentMarginVersion, timestamp int64) *model.AgentMarginVersion {
	var effective *model.AgentMarginVersion
	for index := range versions {
		if versions[index].EffectiveFrom > timestamp {
			break
		}
		effective = &versions[index]
	}
	return effective
}

func sumAgentSettlements(tx *gorm.DB, agentUserID int) (int, error) {
	var total int
	err := tx.Model(&model.AgentSettlement{}).Where("agent_user_id = ?", agentUserID).
		Select("COALESCE(SUM(agent_earnings_quota), 0)").Scan(&total).Error
	return total, err
}

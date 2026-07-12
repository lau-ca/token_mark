package model

import (
	"errors"
	"math"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAgentNotFound       = errors.New("agent not found")
	ErrAgentInvalidRate    = errors.New("agent rate must be between 0 and 1")
	ErrAgentGroupRequired  = errors.New("at least one agent group margin is required")
	ErrAgentDuplicateGroup = errors.New("duplicate agent group margin")
)

type AgentProfile struct {
	UserID                int     `json:"user_id" gorm:"primaryKey;column:user_id"`
	Enabled               bool    `json:"enabled" gorm:"column:enabled;index"`
	PlatformRetentionRate float64 `json:"platform_retention_rate" gorm:"column:platform_retention_rate"`
	Remark                string  `json:"remark" gorm:"type:varchar(255);column:remark"`
	CreatedBy             int     `json:"created_by" gorm:"column:created_by"`
	UpdatedBy             int     `json:"updated_by" gorm:"column:updated_by"`
	CreatedAt             int64   `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt             int64   `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type AgentMarginVersion struct {
	ID                    int                `json:"id" gorm:"primaryKey;column:id"`
	AgentUserID           int                `json:"agent_user_id" gorm:"index:idx_agent_margin_effective,priority:1;uniqueIndex:idx_agent_margin_effective_unique,priority:1;column:agent_user_id"`
	PlatformRetentionRate float64            `json:"platform_retention_rate" gorm:"column:platform_retention_rate"`
	EffectiveFrom         int64              `json:"effective_from" gorm:"index:idx_agent_margin_effective,priority:2;uniqueIndex:idx_agent_margin_effective_unique,priority:2;column:effective_from"`
	CreatedBy             int                `json:"created_by" gorm:"column:created_by"`
	CreatedAt             int64              `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	GroupMargins          []AgentGroupMargin `json:"group_margins,omitempty" gorm:"foreignKey:VersionID"`
}

type AgentGroupMargin struct {
	ID              int     `json:"id" gorm:"primaryKey;column:id"`
	VersionID       int     `json:"version_id" gorm:"uniqueIndex:idx_agent_version_group,priority:1;column:version_id"`
	Group           string  `json:"group" gorm:"type:varchar(64);uniqueIndex:idx_agent_version_group,priority:2;column:group_name"`
	GrossMarginRate float64 `json:"gross_margin_rate" gorm:"column:gross_margin_rate"`
}

type AgentCustomerAssignment struct {
	ID             int    `json:"id" gorm:"primaryKey;column:id"`
	CustomerUserID int    `json:"customer_user_id" gorm:"index:idx_agent_customer_period,priority:1;column:customer_user_id"`
	AgentUserID    int    `json:"agent_user_id" gorm:"index:idx_agent_assignment_period,priority:1;column:agent_user_id"`
	EffectiveFrom  int64  `json:"effective_from" gorm:"index:idx_agent_assignment_period,priority:2;column:effective_from"`
	EffectiveTo    *int64 `json:"effective_to" gorm:"index;column:effective_to"`
	CreatedBy      int    `json:"created_by" gorm:"column:created_by"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

type AgentSettlement struct {
	ID                    int    `json:"id" gorm:"primaryKey;column:id"`
	AgentUserID           int    `json:"agent_user_id" gorm:"index:idx_agent_settlement_cutoff,priority:1;uniqueIndex:idx_agent_settlement_unique,priority:1;column:agent_user_id"`
	PeriodStart           int64  `json:"period_start" gorm:"column:period_start"`
	PeriodEnd             int64  `json:"period_end" gorm:"index:idx_agent_settlement_cutoff,priority:2;uniqueIndex:idx_agent_settlement_unique,priority:2;column:period_end"`
	ConsumptionQuota      int    `json:"consumption_quota" gorm:"column:consumption_quota"`
	GrossProfitQuota      int    `json:"gross_profit_quota" gorm:"column:gross_profit_quota"`
	PlatformRetainedQuota int    `json:"platform_retained_quota" gorm:"column:platform_retained_quota"`
	AgentEarningsQuota    int    `json:"agent_earnings_quota" gorm:"column:agent_earnings_quota"`
	PaymentReference      string `json:"payment_reference" gorm:"type:varchar(255);column:payment_reference"`
	ConfirmedBy           int    `json:"confirmed_by" gorm:"column:confirmed_by"`
	ConfirmedAt           int64  `json:"confirmed_at" gorm:"column:confirmed_at"`
}

type AgentGroupMarginInput struct {
	Group           string  `json:"group"`
	GrossMarginRate float64 `json:"gross_margin_rate"`
}

type AgentProfileView struct {
	AgentProfile
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        int    `json:"role"`
}

func validateAgentRate(rate float64) error {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
		return ErrAgentInvalidRate
	}
	return nil
}

func normalizeAgentGroupMargins(inputs []AgentGroupMarginInput) ([]AgentGroupMarginInput, error) {
	if len(inputs) == 0 {
		return nil, ErrAgentGroupRequired
	}
	result := make([]AgentGroupMarginInput, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		group := strings.TrimSpace(input.Group)
		if group == "" {
			return nil, ErrAgentGroupRequired
		}
		if _, ok := seen[group]; ok {
			return nil, ErrAgentDuplicateGroup
		}
		if err := validateAgentRate(input.GrossMarginRate); err != nil {
			return nil, err
		}
		seen[group] = struct{}{}
		result = append(result, AgentGroupMarginInput{Group: group, GrossMarginRate: input.GrossMarginRate})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Group < result[j].Group })
	return result, nil
}

func GetAgentProfile(tx *gorm.DB, userID int) (*AgentProfile, error) {
	var profile AgentProfile
	if err := tx.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return &profile, nil
}

func GetAgentProfileForUpdate(tx *gorm.DB, userID int) (*AgentProfile, error) {
	var profile AgentProfile
	if err := lockForUpdate(tx).Where("user_id = ?", userID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return &profile, nil
}

func IsEnabledAgent(tx *gorm.DB, userID int) bool {
	if userID <= 0 {
		return false
	}
	var count int64
	return tx.Model(&AgentProfile{}).Where("user_id = ? AND enabled = ?", userID, true).Count(&count).Error == nil && count == 1
}

func SaveAgentConfigTx(tx *gorm.DB, userID int, enabled bool, retentionRate float64, remark string, margins []AgentGroupMarginInput, actorID int, effectiveAt int64) error {
	if err := validateAgentRate(retentionRate); err != nil {
		return err
	}
	var normalized []AgentGroupMarginInput
	var err error
	if enabled {
		normalized, err = normalizeAgentGroupMargins(margins)
		if err != nil {
			return err
		}
		var latest AgentMarginVersion
		if queryErr := tx.Where("agent_user_id = ?", userID).Order("effective_from desc, id desc").First(&latest).Error; queryErr == nil && effectiveAt <= latest.EffectiveFrom {
			effectiveAt = latest.EffectiveFrom + 1
		} else if queryErr != nil && !errors.Is(queryErr, gorm.ErrRecordNotFound) {
			return queryErr
		}
	}

	var existing AgentProfile
	exists := tx.Where("user_id = ?", userID).First(&existing).Error == nil
	profile := AgentProfile{UserID: userID, Enabled: enabled, PlatformRetentionRate: retentionRate, Remark: strings.TrimSpace(remark), CreatedBy: actorID, UpdatedBy: actorID}
	if exists {
		profile.CreatedBy = existing.CreatedBy
		profile.CreatedAt = existing.CreatedAt
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "platform_retention_rate", "remark", "updated_by", "updated_at"}),
	}).Create(&profile).Error; err != nil {
		return err
	}

	if enabled {
		version := AgentMarginVersion{AgentUserID: userID, PlatformRetentionRate: retentionRate, EffectiveFrom: effectiveAt, CreatedBy: actorID}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		rows := make([]AgentGroupMargin, 0, len(normalized))
		for _, margin := range normalized {
			rows = append(rows, AgentGroupMargin{VersionID: version.ID, Group: margin.Group, GrossMarginRate: margin.GrossMarginRate})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}

	if enabled && (!exists || !existing.Enabled) {
		var invitees []User
		if err := tx.Select("id").Where("inviter_id = ?", userID).Find(&invitees).Error; err != nil {
			return err
		}
		for _, invitee := range invitees {
			if err := openAgentAssignmentTx(tx, invitee.Id, userID, actorID, effectiveAt); err != nil {
				return err
			}
		}
	}
	if exists && existing.Enabled && !enabled {
		return closeAgentAssignmentsTx(tx, 0, userID, effectiveAt)
	}
	return nil
}

func openAgentAssignmentTx(tx *gorm.DB, customerUserID int, agentUserID int, actorID int, effectiveAt int64) error {
	return tx.Create(&AgentCustomerAssignment{CustomerUserID: customerUserID, AgentUserID: agentUserID, EffectiveFrom: effectiveAt, CreatedBy: actorID}).Error
}

func closeAgentAssignmentsTx(tx *gorm.DB, customerUserID int, agentUserID int, effectiveAt int64) error {
	query := tx.Model(&AgentCustomerAssignment{}).Where("effective_to IS NULL")
	if customerUserID > 0 {
		query = query.Where("customer_user_id = ?", customerUserID)
	}
	if agentUserID > 0 {
		query = query.Where("agent_user_id = ?", agentUserID)
	}
	return query.Update("effective_to", effectiveAt).Error
}

func UpdateAgentAssignmentForInviterTx(tx *gorm.DB, customerUserID int, inviterID int, actorID int, effectiveAt int64) error {
	if err := closeAgentAssignmentsTx(tx, customerUserID, 0, effectiveAt); err != nil {
		return err
	}
	if !IsEnabledAgent(tx, inviterID) {
		return nil
	}
	return openAgentAssignmentTx(tx, customerUserID, inviterID, actorID, effectiveAt)
}

func GetAgentMarginVersions(tx *gorm.DB, agentUserID int) ([]AgentMarginVersion, error) {
	var versions []AgentMarginVersion
	err := tx.Preload("GroupMargins").Where("agent_user_id = ?", agentUserID).Order("effective_from asc, id asc").Find(&versions).Error
	return versions, err
}

func ListAgentProfiles(tx *gorm.DB, keyword string, includeDisabled bool) ([]AgentProfileView, error) {
	rows := make([]AgentProfileView, 0)
	query := tx.Table("agent_profiles ap").Select("ap.*, u.username, u.display_name, u.role").Joins("JOIN users u ON u.id = ap.user_id")
	if !includeDisabled {
		query = query.Where("ap.enabled = ?", true)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("u.username LIKE ? OR u.display_name LIKE ?", pattern, pattern)
	}
	err := query.Order("ap.enabled desc, ap.updated_at desc").Find(&rows).Error
	return rows, err
}

func AttachAgentEnabled(tx *gorm.DB, users []*User) error {
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	if len(ids) == 0 {
		return nil
	}
	var profiles []AgentProfile
	if err := tx.Select("user_id, enabled").Where("user_id IN ?", ids).Find(&profiles).Error; err != nil {
		return err
	}
	enabled := make(map[int]bool, len(profiles))
	for _, profile := range profiles {
		enabled[profile.UserID] = profile.Enabled
	}
	for _, user := range users {
		user.AgentEnabled = enabled[user.Id]
	}
	return nil
}

func ListAgentSettlements(tx *gorm.DB, agentUserID int) ([]AgentSettlement, error) {
	var rows []AgentSettlement
	err := tx.Where("agent_user_id = ?", agentUserID).Order("period_end desc, id desc").Find(&rows).Error
	return rows, err
}

func GetLatestAgentSettlement(tx *gorm.DB, agentUserID int, forUpdate bool) (*AgentSettlement, error) {
	var settlement AgentSettlement
	query := tx.Where("agent_user_id = ?", agentUserID).Order("period_end desc, id desc")
	if forUpdate {
		query = lockForUpdate(query)
	}
	err := query.First(&settlement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &settlement, err
}

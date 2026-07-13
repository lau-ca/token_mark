package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrAgentSettlementCutoff   = errors.New("invalid agent settlement cutoff")
	ErrAgentNoUnsettledEarning = errors.New("agent has no unsettled earnings")
)

const AgentSettlementSafetyDelaySeconds int64 = 60

type AgentSettlementPreview struct {
	PeriodStart           int64 `json:"period_start"`
	PeriodEnd             int64 `json:"period_end"`
	ConsumptionQuota      int   `json:"consumption_quota"`
	GrossProfitQuota      int   `json:"gross_profit_quota"`
	PlatformRetainedQuota int   `json:"platform_retained_quota"`
	AgentEarningsQuota    int   `json:"agent_earnings_quota"`
}

func PreviewAgentSettlement(tx *gorm.DB, agentUserID int, cutoff int64) (*AgentSettlementPreview, error) {
	profile, err := model.GetAgentProfile(tx, agentUserID)
	if err != nil {
		return nil, err
	}
	latest, err := model.GetLatestAgentSettlement(tx, agentUserID, false)
	if err != nil {
		return nil, err
	}
	start := profile.CreatedAt
	if start <= 0 {
		start = 1
	}
	if latest != nil {
		start = latest.PeriodEnd + 1
	}
	if cutoff < start || cutoff > common.GetTimestamp()-AgentSettlementSafetyDelaySeconds {
		return nil, ErrAgentSettlementCutoff
	}
	stats, err := CalculateAgentStats(tx, agentUserID, start, cutoff, "", "", 1, 0)
	if err != nil {
		return nil, err
	}
	return &AgentSettlementPreview{
		PeriodStart:           start,
		PeriodEnd:             cutoff,
		ConsumptionQuota:      stats.Summary.ConsumptionQuota,
		GrossProfitQuota:      stats.Summary.GrossProfitQuota,
		PlatformRetainedQuota: stats.Summary.PlatformRetainedQuota,
		AgentEarningsQuota:    stats.Summary.AgentEarningsQuota,
	}, nil
}

func ConfirmAgentSettlement(agentUserID int, cutoff int64, paymentReference string, actorID int) (*model.AgentSettlement, error) {
	var settlement *model.AgentSettlement
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := model.GetAgentProfileForUpdate(tx, agentUserID); err != nil {
			return err
		}
		preview, err := PreviewAgentSettlement(tx, agentUserID, cutoff)
		if err != nil {
			return err
		}
		if preview.AgentEarningsQuota <= 0 {
			return ErrAgentNoUnsettledEarning
		}
		settlement = &model.AgentSettlement{
			AgentUserID:           agentUserID,
			PeriodStart:           preview.PeriodStart,
			PeriodEnd:             preview.PeriodEnd,
			ConsumptionQuota:      preview.ConsumptionQuota,
			GrossProfitQuota:      preview.GrossProfitQuota,
			PlatformRetainedQuota: preview.PlatformRetainedQuota,
			AgentEarningsQuota:    preview.AgentEarningsQuota,
			PaymentReference:      strings.TrimSpace(paymentReference),
			ConfirmedBy:           actorID,
			ConfirmedAt:           common.GetTimestamp(),
		}
		if len(settlement.PaymentReference) > 255 {
			return errors.New("payment reference is too long")
		}
		return tx.Create(settlement).Error
	})
	return settlement, err
}

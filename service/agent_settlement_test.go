package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSettlementPreviewAndConfirm(t *testing.T) {
	setupAgentStatsTest(t)
	require.NoError(t, model.DB.Create(&model.AgentProfile{UserID: 10, Enabled: true, PlatformRetentionRate: 0.1, CreatedAt: 1000}).Error)
	require.NoError(t, model.DB.Create(&model.AgentMarginVersion{ID: 1, AgentUserID: 10, PlatformRetentionRate: 0.1, EffectiveFrom: 1000}).Error)
	require.NoError(t, model.DB.Create(&model.AgentGroupMargin{VersionID: 1, Group: "codex", GrossMarginRate: 0.3}).Error)
	require.NoError(t, model.DB.Create(&model.AgentCustomerAssignment{CustomerUserID: 20, AgentUserID: 10, EffectiveFrom: 1000}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 20, Username: "customer", UseGroup: "codex", CreatedAt: 1500, Quota: 10000}).Error)

	preview, err := PreviewAgentSettlement(model.DB, 10, 2000)
	require.NoError(t, err)
	assert.Equal(t, 2700, preview.AgentEarningsQuota)

	settlement, err := ConfirmAgentSettlement(10, 2000, "offline-001", 1)
	require.NoError(t, err)
	assert.Equal(t, preview.AgentEarningsQuota, settlement.AgentEarningsQuota)

	_, err = ConfirmAgentSettlement(10, 2000, "duplicate", 1)
	require.ErrorIs(t, err, ErrAgentSettlementCutoff)
}

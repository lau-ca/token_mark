package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAgentStatsTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.QuotaData{}, &model.AgentProfile{}, &model.AgentMarginVersion{}, &model.AgentGroupMargin{}, &model.AgentCustomerAssignment{}, &model.AgentSettlement{}))
	for _, table := range []string{"quota_data", "agent_group_margins", "agent_margin_versions", "agent_customer_assignments", "agent_settlements", "agent_profiles", "users"} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
}

func TestAgentStatsUsesAssignmentAndVersionPeriods(t *testing.T) {
	setupAgentStatsTest(t)
	require.NoError(t, model.DB.Create(&model.AgentProfile{UserID: 10, Enabled: true, PlatformRetentionRate: 0.2}).Error)
	versions := []model.AgentMarginVersion{
		{ID: 1, AgentUserID: 10, PlatformRetentionRate: 0.1, EffectiveFrom: 1000},
		{ID: 2, AgentUserID: 10, PlatformRetentionRate: 0.2, EffectiveFrom: 2000},
	}
	require.NoError(t, model.DB.Create(&versions).Error)
	require.NoError(t, model.DB.Create(&[]model.AgentGroupMargin{
		{VersionID: 1, Group: "codex", GrossMarginRate: 0.3},
		{VersionID: 2, Group: "codex", GrossMarginRate: 0.4},
	}).Error)
	end := int64(2500)
	require.NoError(t, model.DB.Create(&model.AgentCustomerAssignment{CustomerUserID: 20, AgentUserID: 10, EffectiveFrom: 1000, EffectiveTo: &end}).Error)
	require.NoError(t, model.DB.Create(&[]model.QuotaData{
		{UserID: 20, Username: "customer", UseGroup: "codex", CreatedAt: 1500, Quota: 10000},
		{UserID: 20, Username: "customer", UseGroup: "codex", CreatedAt: 2200, Quota: 10000},
		{UserID: 20, Username: "customer", UseGroup: "image", CreatedAt: 2300, Quota: 5000},
		{UserID: 20, Username: "customer", UseGroup: "codex", CreatedAt: 2600, Quota: 10000},
	}).Error)

	result, err := CalculateAgentStats(model.DB, 10, 1000, 3000, "", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 25000, result.Summary.ConsumptionQuota)
	assert.Equal(t, 7000, result.Summary.GrossProfitQuota)
	assert.Equal(t, 1100, result.Summary.PlatformRetainedQuota)
	assert.Equal(t, 5900, result.Summary.AgentEarningsQuota)
	assert.Equal(t, 1, result.Summary.CustomerCount)
	require.Len(t, result.Details, 2)
}

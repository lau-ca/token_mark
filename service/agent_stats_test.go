package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAgentStatsTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.AgentProfile{}, &model.AgentMarginVersion{}, &model.AgentGroupMargin{}, &model.AgentCustomerAssignment{}, &model.AgentSettlement{}))
	require.NoError(t, model.LOG_DB.AutoMigrate(&model.Log{}))
	for _, table := range []string{"agent_group_margins", "agent_margin_versions", "agent_customer_assignments", "agent_settlements", "agent_profiles", "users"} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
	require.NoError(t, model.LOG_DB.Exec("DELETE FROM logs").Error)
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
		{VersionID: 1, Group: "codex", GrossMarginRate: 0.3, PlatformRetentionRate: float64Pointer(0.1)},
		{VersionID: 2, Group: "codex", GrossMarginRate: 0.4, PlatformRetentionRate: float64Pointer(0.2)},
		{VersionID: 2, Group: "image", GrossMarginRate: 0.12, PlatformRetentionRate: float64Pointer(0.19)},
	}).Error)
	end := int64(2500)
	require.NoError(t, model.DB.Create(&model.AgentCustomerAssignment{CustomerUserID: 20, AgentUserID: 10, EffectiveFrom: 1000, EffectiveTo: &end}).Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 20, Username: "customer", Group: "codex", CreatedAt: 1500, Quota: 10000, Type: model.LogTypeConsume},
		{UserId: 20, Username: "customer", Group: "codex", CreatedAt: 2200, Quota: 10000, Type: model.LogTypeConsume},
		{UserId: 20, Username: "customer", Group: "image", CreatedAt: 2300, Quota: 5000, Type: model.LogTypeConsume},
		{UserId: 20, Username: "customer", Group: "codex", CreatedAt: 2600, Quota: 10000, Type: model.LogTypeConsume},
	}).Error)

	result, err := CalculateAgentStats(model.DB, 10, 1000, 3000, "", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 25000, result.Summary.ConsumptionQuota)
	assert.Equal(t, 7600, result.Summary.GrossProfitQuota)
	assert.Equal(t, 1214, result.Summary.PlatformRetainedQuota)
	assert.Equal(t, 6386, result.Summary.AgentEarningsQuota)
	assert.Equal(t, 1, result.Summary.CustomerCount)
	require.Len(t, result.Details, 2)
}

func TestAgentStatsRoundsAfterAggregatingConfigurationPeriod(t *testing.T) {
	setupAgentStatsTest(t)
	require.NoError(t, model.DB.Create(&model.AgentProfile{UserID: 10, Enabled: true}).Error)
	require.NoError(t, model.DB.Create(&model.AgentMarginVersion{ID: 1, AgentUserID: 10, EffectiveFrom: 1000}).Error)
	require.NoError(t, model.DB.Create(&model.AgentGroupMargin{VersionID: 1, Group: "codex", GrossMarginRate: 0.3, PlatformRetentionRate: float64Pointer(0.1)}).Error)
	require.NoError(t, model.DB.Create(&model.AgentCustomerAssignment{CustomerUserID: 20, AgentUserID: 10, EffectiveFrom: 1000}).Error)
	for timestamp := int64(1100); timestamp < 1104; timestamp++ {
		require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 20, Username: "customer", Group: "codex", CreatedAt: timestamp, Quota: 1, Type: model.LogTypeConsume}).Error)
	}

	result, err := CalculateAgentStats(model.DB, 10, 1000, 1200, "", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 4, result.Summary.ConsumptionQuota)
	assert.Equal(t, 1, result.Summary.GrossProfitQuota)
	assert.Equal(t, 1, result.Summary.AgentEarningsQuota)
}

func float64Pointer(value float64) *float64 {
	return &value
}

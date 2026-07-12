package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAgentTestState(t *testing.T) {
	t.Helper()
	setupUserUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&AgentProfile{}, &AgentMarginVersion{}, &AgentGroupMargin{}, &AgentCustomerAssignment{}, &AgentSettlement{}))
	for _, table := range []string{"agent_group_margins", "agent_margin_versions", "agent_customer_assignments", "agent_settlements", "agent_profiles"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
}

func TestAgentConfigCreatesVersionAndAssignments(t *testing.T) {
	setupAgentTestState(t)
	require.NoError(t, DB.Create(&User{Id: 10, Username: "agent", Password: "password", AffCode: "agent-code", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&User{Id: 20, Username: "customer", Password: "password", AffCode: "customer-code", InviterId: 10, Status: common.UserStatusEnabled}).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		return SaveAgentConfigTx(tx, 10, true, 0.09, "sales", []AgentGroupMarginInput{
			{Group: "codex", GrossMarginRate: 0.3},
			{Group: "image", GrossMarginRate: 0.12},
		}, 1, 1000)
	})
	require.NoError(t, err)

	profile, err := GetAgentProfile(DB, 10)
	require.NoError(t, err)
	assert.True(t, profile.Enabled)
	assert.InDelta(t, 0.09, profile.PlatformRetentionRate, 0.000001)

	versions, err := GetAgentMarginVersions(DB, 10)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.Len(t, versions[0].GroupMargins, 2)

	var assignment AgentCustomerAssignment
	require.NoError(t, DB.Where("customer_user_id = ?", 20).First(&assignment).Error)
	assert.Equal(t, 10, assignment.AgentUserID)
	assert.Equal(t, int64(1000), assignment.EffectiveFrom)
}

func TestAgentConfigRejectsInvalidAndDuplicateRates(t *testing.T) {
	setupAgentTestState(t)
	err := SaveAgentConfigTx(DB, 10, true, 1.1, "", []AgentGroupMarginInput{{Group: "codex", GrossMarginRate: 0.3}}, 1, 1000)
	require.ErrorIs(t, err, ErrAgentInvalidRate)
	err = SaveAgentConfigTx(DB, 10, true, 0.1, "", []AgentGroupMarginInput{
		{Group: "codex", GrossMarginRate: 0.3},
		{Group: "codex", GrossMarginRate: 0.2},
	}, 1, 1000)
	require.ErrorIs(t, err, ErrAgentDuplicateGroup)
}

func TestAgentAssignmentChangesOnlyFuturePeriod(t *testing.T) {
	setupAgentTestState(t)
	require.NoError(t, DB.Create(&AgentProfile{UserID: 10, Enabled: true}).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserID: 11, Enabled: true}).Error)
	require.NoError(t, UpdateAgentAssignmentForInviterTx(DB, 20, 10, 1, 1000))
	require.NoError(t, UpdateAgentAssignmentForInviterTx(DB, 20, 11, 1, 2000))

	var rows []AgentCustomerAssignment
	require.NoError(t, DB.Where("customer_user_id = ?", 20).Order("effective_from asc").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].EffectiveTo)
	assert.Equal(t, int64(2000), *rows[0].EffectiveTo)
	assert.Nil(t, rows[1].EffectiveTo)
	assert.Equal(t, 11, rows[1].AgentUserID)
}

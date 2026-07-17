package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelUpdateClearsBalanceSettings(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}))
	channel := &Channel{
		Id:              987654,
		Type:            1,
		Key:             "sk-test",
		Name:            "balance-settings-test",
		BalancePlatform: "new_api",
		BalanceBaseURL:  "https://example.com",
		BalanceUserID:   1787,
		BalanceAuthKey:  "account-token",
	}
	require.NoError(t, DB.Create(channel).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("id = ?", channel.Id).Delete(&Channel{}).Error)
		require.NoError(t, DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error)
	})

	channel.BalancePlatform = ""
	channel.BalanceBaseURL = ""
	channel.BalanceUserID = 0
	channel.BalanceAuthKey = ""
	require.NoError(t, channel.Update())

	stored, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Empty(t, stored.BalancePlatform)
	assert.Empty(t, stored.BalanceBaseURL)
	assert.Zero(t, stored.BalanceUserID)
	assert.Empty(t, stored.BalanceAuthKey)
}

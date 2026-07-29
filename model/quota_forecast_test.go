package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetQuotaForecastUsersAndUsage(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&User{}, &QuotaData{}))

	users := []User{
		{Id: 101, Username: "forecast-a", Password: "password", AffCode: "forecast-a", Status: common.UserStatusEnabled, Quota: 900, CreatedAt: 100},
		{Id: 102, Username: "forecast-b", Password: "password", AffCode: "forecast-b", Status: common.UserStatusEnabled, Quota: 700, CreatedAt: 200},
	}
	require.NoError(t, DB.Create(&users).Error)
	require.NoError(t, DB.Create(&[]QuotaData{
		{UserID: 101, Username: "forecast-a", ModelName: "gpt-a", CreatedAt: 1000, Quota: 40},
		{UserID: 101, Username: "forecast-a", ModelName: "gpt-b", CreatedAt: 1000, Quota: 60},
		{UserID: 101, Username: "forecast-a", ModelName: "gpt-a", CreatedAt: 2000, Quota: 25},
		{UserID: 102, Username: "forecast-b", ModelName: "gpt-a", CreatedAt: 3000, Quota: 80},
	}).Error)

	forecastUsers, err := GetQuotaForecastUsers([]int{101, 102})
	require.NoError(t, err)
	require.Len(t, forecastUsers, 2)
	assert.Equal(t, QuotaForecastUser{UserID: 101, Quota: 900, CreatedAt: 100}, forecastUsers[0])
	assert.Equal(t, QuotaForecastUser{UserID: 102, Quota: 700, CreatedAt: 200}, forecastUsers[1])

	usage, err := GetQuotaForecastUsage([]int{101, 102}, 900, 2500)
	require.NoError(t, err)
	require.Len(t, usage, 2)
	assert.Equal(t, QuotaForecastUsage{UserID: 101, CreatedAt: 1000, Quota: 100}, usage[0])
	assert.Equal(t, QuotaForecastUsage{UserID: 101, CreatedAt: 2000, Quota: 25}, usage[1])
}

func TestGetQuotaForecastUsageReturnsEmptyRowsForNoUsage(t *testing.T) {
	truncateTables(t)
	usage, err := GetQuotaForecastUsage([]int{999}, 1000, 2000)
	require.NoError(t, err)
	assert.Empty(t, usage)
}

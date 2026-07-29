package service

import (
	"context"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateQuotaForecastUsesRecentWeightedWindows(t *testing.T) {
	const now = int64(8 * 24 * 60 * 60)
	usage := make([]model.QuotaForecastUsage, 0, 7)
	for index := 0; index < 7; index++ {
		usage = append(usage, model.QuotaForecastUsage{
			UserID:    1,
			CreatedAt: now - int64(index)*quotaForecastDaySeconds - 1,
			Quota:     int64((index + 1) * 100),
		})
	}

	result := calculateQuotaForecast(model.QuotaForecastUser{UserID: 1, Quota: 1000, CreatedAt: 1}, usage, now)

	require.Equal(t, QuotaForecastStatusPredicted, result.Status)
	assert.InDelta(t, 300, result.WeightedDailyUsage, 0.001)
	assert.Equal(t, int64(288000), result.RemainingSeconds)
	assert.Equal(t, now+288000, result.PredictedExhaustedAt)
}

func TestCalculateQuotaForecastProratesPartialOldestWindow(t *testing.T) {
	const now = int64(8 * 24 * 60 * 60)
	createdAt := now - 6*quotaForecastDaySeconds - quotaForecastDaySeconds/2
	usage := []model.QuotaForecastUsage{{UserID: 1, CreatedAt: createdAt + 1, Quota: 50}}

	result := calculateQuotaForecast(model.QuotaForecastUser{UserID: 1, Quota: 1000, CreatedAt: createdAt}, usage, now)

	require.Equal(t, QuotaForecastStatusPredicted, result.Status)
	assert.InDelta(t, 50.0/27.5, result.WeightedDailyUsage, 0.001)
}

func TestCalculateQuotaForecastSpecialStates(t *testing.T) {
	const now = int64(10 * 24 * 60 * 60)
	tests := []struct {
		name   string
		user   model.QuotaForecastUser
		usage  []model.QuotaForecastUsage
		status string
	}{
		{name: "depleted", user: model.QuotaForecastUser{UserID: 1, Quota: 0, CreatedAt: 1}, status: QuotaForecastStatusDepleted},
		{name: "sampling", user: model.QuotaForecastUser{UserID: 1, Quota: 100, CreatedAt: now - 3600}, status: QuotaForecastStatusSampling},
		{name: "no recent usage", user: model.QuotaForecastUser{UserID: 1, Quota: 100, CreatedAt: 1}, status: QuotaForecastStatusNoRecentUsage},
		{name: "ignores stale usage", user: model.QuotaForecastUser{UserID: 1, Quota: 100, CreatedAt: 1}, usage: []model.QuotaForecastUsage{{UserID: 1, CreatedAt: now - 8*quotaForecastDaySeconds, Quota: 100}}, status: QuotaForecastStatusNoRecentUsage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := calculateQuotaForecast(test.user, test.usage, now)
			assert.Equal(t, test.status, result.Status)
			assert.Zero(t, result.PredictedExhaustedAt)
		})
	}
}

func TestCalculateQuotaForecastRejectsTimestampOverflow(t *testing.T) {
	result := calculateQuotaForecast(
		model.QuotaForecastUser{UserID: 1, Quota: math.MaxInt, CreatedAt: 1},
		[]model.QuotaForecastUsage{{UserID: 1, CreatedAt: math.MaxInt64 - quotaForecastDaySeconds, Quota: 1}},
		math.MaxInt64-1,
	)
	assert.Equal(t, QuotaForecastStatusUnavailable, result.Status)
}

func TestGetQuotaForecastsInvalidatesCacheWhenQuotaChanges(t *testing.T) {
	quotaForecastCache.Lock()
	quotaForecastCache.entries = make(map[int]quotaForecastCacheEntry)
	quotaForecastCache.Unlock()
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.QuotaData{}))
	require.NoError(t, model.DB.Exec("DELETE FROM quota_data").Error)
	require.NoError(t, model.DB.Unscoped().Where("id = ?", 9876).Delete(&model.User{}).Error)
	user := model.User{Id: 9876, Username: "forecast-cache", Password: "password", AffCode: "forecast-cache", Status: 1, Quota: 1000, CreatedAt: 1}
	require.NoError(t, model.DB.Create(&user).Error)
	const now = int64(10 * 24 * 60 * 60)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: user.Id, Username: user.Username, ModelName: "gpt", CreatedAt: now - 1, Quota: 100}).Error)

	first, err := GetQuotaForecasts(context.Background(), []int{user.Id}, now)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 2000).Error)
	second, err := GetQuotaForecasts(context.Background(), []int{user.Id}, now)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Greater(t, second[0].RemainingSeconds, first[0].RemainingSeconds)
}

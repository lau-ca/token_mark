package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNewModelSuccessRateWindowCalculatesBothDenominators(t *testing.T) {
	window := newModelSuccessRateWindow(model.ModelSuccessRateCounts{
		SuccessCount:     80,
		ClientErrorCount: 10,
		ServerErrorCount: 5,
		OtherErrorCount:  5,
	})

	require.NotNil(t, window.ErrorRateIncluding4xx)
	require.NotNil(t, window.ErrorRateExcluding4xx)
	assert.Equal(t, 20.0, *window.ErrorRateIncluding4xx)
	assert.Equal(t, 11.1111, *window.ErrorRateExcluding4xx)
}

func TestNewModelSuccessRateWindowUsesNullRatesWithoutRequests(t *testing.T) {
	window := newModelSuccessRateWindow(model.ModelSuccessRateCounts{})

	assert.Nil(t, window.ErrorRateIncluding4xx)
	assert.Nil(t, window.ErrorRateExcluding4xx)
}

func TestGetModelSuccessRateAtUsesBeijingDayAndRollingFiveMinutes(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	originalLogDB := model.LOG_DB
	originalLogDatabaseType := common.LogDatabaseType()
	model.LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		common.SetLogDatabaseType(originalLogDatabaseType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Date(2026, 7, 28, 3, 4, 5, 0, time.UTC)
	beijingDayStart := time.Date(2026, 7, 28, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)).Unix()
	require.NoError(t, db.Create(&[]model.Log{
		{CreatedAt: beijingDayStart, Type: model.LogTypeConsume, ModelName: "window-model", RequestId: "day-start", Other: `{}`},
		{CreatedAt: now.Unix() - 300, Type: model.LogTypeError, ModelName: "window-model", RequestId: "five-minute-start", Other: `{"status_code":500}`},
		{CreatedAt: now.Unix() - 301, Type: model.LogTypeConsume, ModelName: "window-model", RequestId: "before-five-minutes", Other: `{}`},
	}).Error)

	response, err := getModelSuccessRateAt("window-model", now)
	require.NoError(t, err)
	require.NotNil(t, response.Today.ErrorRateIncluding4xx)
	require.NotNil(t, response.Last5Minutes.ErrorRateIncluding4xx)
	assert.Equal(t, 33.3333, *response.Today.ErrorRateIncluding4xx)
	assert.Equal(t, 100.0, *response.Last5Minutes.ErrorRateIncluding4xx)
}

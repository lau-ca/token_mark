package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelSuccessRateTest(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))

	originalLogDB := LOG_DB
	originalLogDatabaseType := common.LogDatabaseType()
	LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		LOG_DB = originalLogDB
		common.SetLogDatabaseType(originalLogDatabaseType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetModelSuccessRateCountsDeduplicatesFinalRequestOutcome(t *testing.T) {
	db := setupModelSuccessRateTest(t)
	logs := []Log{
		{CreatedAt: 100, Type: LogTypeConsume, ModelName: "gpt-image-2-w", RequestId: "success", Other: `{}`},
		{CreatedAt: 101, Type: LogTypeConsume, ModelName: "gpt-image-2-w", RequestId: "success", Other: `{}`},
		{CreatedAt: 102, Type: LogTypeError, ModelName: "gpt-image-2-w", RequestId: "retried", Other: `{"status_code":502}`},
		{CreatedAt: 103, Type: LogTypeConsume, ModelName: "gpt-image-2-w", RequestId: "retried", Other: `{}`},
		{CreatedAt: 104, Type: LogTypeError, ModelName: "gpt-image-2-w", RequestId: "client", Other: `{"status_code":502}`},
		{CreatedAt: 105, Type: LogTypeError, ModelName: "gpt-image-2-w", RequestId: "client", Other: `{"status_code":"400"}`},
		{CreatedAt: 106, Type: LogTypeError, ModelName: "gpt-image-2-w", RequestId: "server", Other: `{"status_code":504}`},
		{CreatedAt: 107, Type: LogTypeError, ModelName: "gpt-image-2-w", RequestId: "other", Other: `{"message":"timeout"}`},
		{CreatedAt: 108, Type: LogTypeError, ModelName: "another-model", RequestId: "different-model", Other: `{"status_code":500}`},
		{CreatedAt: 99, Type: LogTypeError, ModelName: "gpt-image-2-w", RequestId: "before-window", Other: `{"status_code":500}`},
		{CreatedAt: 111, Type: LogTypeError, ModelName: "gpt-image-2-w", RequestId: "after-window", Other: `{"status_code":500}`},
	}
	require.NoError(t, db.Create(&logs).Error)

	counts, err := GetModelSuccessRateCounts("gpt-image-2-w", 100, 110)
	require.NoError(t, err)

	assert.Equal(t, int64(2), counts.SuccessCount)
	assert.Equal(t, int64(1), counts.ClientErrorCount)
	assert.Equal(t, int64(1), counts.ServerErrorCount)
	assert.Equal(t, int64(1), counts.OtherErrorCount)
}

func TestGetModelSuccessRateCountsIncludesWindowBoundaries(t *testing.T) {
	db := setupModelSuccessRateTest(t)
	require.NoError(t, db.Create(&[]Log{
		{CreatedAt: 100, Type: LogTypeConsume, ModelName: "boundary-model", RequestId: "at-start", Other: `{}`},
		{CreatedAt: 200, Type: LogTypeError, ModelName: "boundary-model", RequestId: "at-end", Other: `{"status_code":503}`},
	}).Error)

	counts, err := GetModelSuccessRateCounts("boundary-model", 100, 200)
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts.SuccessCount)
	assert.Equal(t, int64(1), counts.ServerErrorCount)
}

package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type keyUsageExportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		GeneratedAt    int64                       `json:"generated_at"`
		StartTimestamp int64                       `json:"start_timestamp"`
		EndTimestamp   int64                       `json:"end_timestamp"`
		Keys           []model.KeyUsageExportKey   `json:"keys"`
		Models         []model.KeyUsageExportModel `json:"models"`
	} `json:"data"`
}

func setupKeyUsageExportControllerTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Log{}))
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, model.DB.Create(&model.Token{
		Id:     11,
		UserId: 1,
		Key:    "primary-secret-key",
		Name:   "primary",
		Status: common.TokenStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:     22,
		UserId: 2,
		Key:    "other-user-key",
		Name:   "other",
		Status: common.TokenStatusEnabled,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId:           1,
		Type:             model.LogTypeConsume,
		TokenId:          11,
		TokenName:        "primary",
		ModelName:        "gpt-5",
		PromptTokens:     100,
		CompletionTokens: 20,
		Quota:            300,
		CreatedAt:        1200,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId:           2,
		Type:             model.LogTypeConsume,
		TokenId:          22,
		TokenName:        "other",
		ModelName:        "gpt-5",
		PromptTokens:     900,
		CompletionTokens: 90,
		Quota:            900,
		CreatedAt:        1300,
	}).Error)
}

func decodeKeyUsageExportResponse(t *testing.T, recorder *httptest.ResponseRecorder) keyUsageExportResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload keyUsageExportResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func TestGetUserKeyUsageExportRestrictsToAuthenticatedUser(t *testing.T) {
	setupKeyUsageExportControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/keys/self/export?start_timestamp=1000&end_timestamp=2000", nil)

	GetUserKeyUsageExport(ctx)

	payload := decodeKeyUsageExportResponse(t, recorder)
	require.True(t, payload.Success, payload.Message)
	assert.Equal(t, int64(1000), payload.Data.StartTimestamp)
	assert.Equal(t, int64(2000), payload.Data.EndTimestamp)
	assert.Positive(t, payload.Data.GeneratedAt)
	require.Len(t, payload.Data.Keys, 1)
	require.Len(t, payload.Data.Models, 1)
	assert.Equal(t, 11, payload.Data.Keys[0].TokenID)
	assert.Equal(t, 120, payload.Data.Keys[0].TotalTokens)
	assert.Equal(t, "gpt-5", payload.Data.Models[0].ModelName)
}

func TestGetUserKeyUsageExportRejectsInvalidAndOversizedRanges(t *testing.T) {
	testCases := []struct {
		name    string
		query   string
		message string
	}{
		{name: "invalid start", query: "start_timestamp=bad&end_timestamp=2000", message: "invalid start_timestamp"},
		{name: "reversed", query: "start_timestamp=2000&end_timestamp=1000", message: "invalid time range"},
		{name: "over one month", query: "start_timestamp=1000&end_timestamp=2593001", message: "时间跨度不能超过 1 个月"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setupKeyUsageExportControllerTestDB(t)

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("id", 1)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/keys/self/export?"+testCase.query, nil)

			GetUserKeyUsageExport(ctx)

			payload := decodeKeyUsageExportResponse(t, recorder)
			assert.False(t, payload.Success)
			assert.Equal(t, testCase.message, payload.Message)
		})
	}
}

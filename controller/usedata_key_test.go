package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type keyQuotaResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Data    []model.KeyQuotaData `json:"data"`
}

func setupKeyQuotaControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.QuotaData{}))
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
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID:    1,
		TokenID:   11,
		CreatedAt: 1100,
		Count:     2,
		Quota:     100,
		TokenUsed: 40,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID:    2,
		TokenID:   22,
		CreatedAt: 1200,
		Count:     1,
		Quota:     70,
		TokenUsed: 30,
	}).Error)
}

func decodeKeyQuotaResponse(t *testing.T, recorder *httptest.ResponseRecorder) keyQuotaResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload keyQuotaResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func TestGetUserKeyQuotaDatesRestrictsToAuthenticatedUser(t *testing.T) {
	setupKeyQuotaControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/keys/self?start_timestamp=1000&end_timestamp=2000", nil)

	GetUserKeyQuotaDates(ctx)

	payload := decodeKeyQuotaResponse(t, recorder)
	require.True(t, payload.Success, payload.Message)
	require.Len(t, payload.Data, 1)
	require.Equal(t, 11, payload.Data[0].TokenID)
	require.Equal(t, 100, payload.Data[0].Quota)
}

func TestGetUserKeyQuotaDatesRejectsInvalidAndOversizedRanges(t *testing.T) {
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
			setupKeyQuotaControllerTestDB(t)

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("id", 1)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/keys/self?"+testCase.query, nil)

			GetUserKeyQuotaDates(ctx)

			payload := decodeKeyQuotaResponse(t, recorder)
			require.False(t, payload.Success)
			require.Equal(t, testCase.message, payload.Message)
		})
	}
}

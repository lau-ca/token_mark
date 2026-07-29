package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeQuotaForecastUserIDs(t *testing.T) {
	ids, message := normalizeQuotaForecastUserIDs([]int{3, 1, 3, 2})
	assert.Empty(t, message)
	assert.Equal(t, []int{3, 1, 2}, ids)

	ids, message = normalizeQuotaForecastUserIDs(nil)
	assert.Nil(t, ids)
	assert.Equal(t, "user_ids is required", message)

	ids, message = normalizeQuotaForecastUserIDs([]int{1, 0})
	assert.Nil(t, ids)
	assert.Equal(t, "user_ids must contain positive integers", message)

	tooMany := make([]int, maxQuotaForecastUserIDs+1)
	for index := range tooMany {
		tooMany[index] = index + 1
	}
	ids, message = normalizeQuotaForecastUserIDs(tooMany)
	assert.Nil(t, ids)
	assert.Equal(t, "user_ids cannot contain more than 100 unique users", message)
}

func TestGetQuotaForecastsRejectsMalformedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/user/quota-forecast", bytes.NewBufferString("{"))

	GetQuotaForecasts(context)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	response := map[string]interface{}{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, false, response["success"])
}

func TestGetSelfQuotaForecastRejectsMissingUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/quota-forecast", nil)

	GetSelfQuotaForecast(context)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

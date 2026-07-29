package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type topUpInfoTestResponse struct {
	Success bool `json:"success"`
	Data    struct {
		EnableInfiniTopUp bool                `json:"enable_infini_topup"`
		PayMethods        []map[string]string `json:"pay_methods"`
	} `json:"data"`
}

func requestTopUpInfoForTest(t *testing.T) topUpInfoTestResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)

	GetTopUpInfo(context)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := topUpInfoTestResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func TestGetTopUpInfoOnlyShowsConfiguredInfiniMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	confirmPaymentComplianceForTest(t)

	originalPayMethods := operation_setting.PayMethods
	originalEnabled := setting.InfiniEnabled
	originalKeyID := setting.InfiniKeyID
	originalSecretKey := setting.InfiniSecretKey
	originalWebhookSecret := setting.InfiniWebhookSecret
	originalInfiniPayMethods := setting.InfiniPayMethods
	originalUnitPrice := setting.InfiniUnitPrice
	originalMinTopUp := setting.InfiniMinTopUp
	t.Cleanup(func() {
		operation_setting.PayMethods = originalPayMethods
		setting.InfiniEnabled = originalEnabled
		setting.InfiniKeyID = originalKeyID
		setting.InfiniSecretKey = originalSecretKey
		setting.InfiniWebhookSecret = originalWebhookSecret
		setting.InfiniPayMethods = originalInfiniPayMethods
		setting.InfiniUnitPrice = originalUnitPrice
		setting.InfiniMinTopUp = originalMinTopUp
	})

	setting.InfiniEnabled = true
	setting.InfiniKeyID = "key-id"
	setting.InfiniSecretKey = "secret"
	setting.InfiniWebhookSecret = "webhook"
	setting.InfiniPayMethods = "[1]"
	setting.InfiniUnitPrice = 0.14
	setting.InfiniMinTopUp = 1

	operation_setting.PayMethods = []map[string]string{{"name": "支付宝", "type": "alipay"}}
	response := requestTopUpInfoForTest(t)
	assert.False(t, response.Data.EnableInfiniTopUp)
	for _, method := range response.Data.PayMethods {
		assert.NotEqual(t, model.PaymentMethodInfini, method["type"])
	}

	configuredInfini := map[string]string{"name": "Infini", "type": model.PaymentMethodInfini}
	operation_setting.PayMethods = append(operation_setting.PayMethods, configuredInfini)
	response = requestTopUpInfoForTest(t)
	assert.True(t, response.Data.EnableInfiniTopUp)
	require.Len(t, response.Data.PayMethods, 2)
	assert.Equal(t, "1", response.Data.PayMethods[1]["min_topup"])
	assert.Empty(t, configuredInfini["min_topup"])
}

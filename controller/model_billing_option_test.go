package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelBillingOptionTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Log{}, &model.User{}))
	savedDB := model.DB
	savedLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	savedRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "root", Status: common.UserStatusEnabled}).Error)

	common.OptionMapRWMutex.Lock()
	savedOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	savedModes := billing_setting.GetBillingModeCopy()
	savedExpressions := billing_setting.GetBillingExprCopy()
	savedModelPrices := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		model.DB = savedDB
		model.LOG_DB = savedLogDB
		common.RedisEnabled = savedRedisEnabled
		billing_setting.ReplaceConfig(savedModes, savedExpressions)
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrices))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = savedOptionMap
		common.OptionMapRWMutex.Unlock()
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func invokeModelBillingOptionUpdate(t *testing.T, request ModelBillingOptionsUpdateRequest) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	payload, err := common.Marshal(request)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/model-billing", bytes.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", 1)
	context.Set("username", "root")

	UpdateModelBillingOptions(context)

	var response map[string]interface{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func invokeGenericOptionUpdate(t *testing.T, key, value string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	payload, err := common.Marshal(OptionUpdateRequest{Key: key, Value: value})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", 1)
	context.Set("username", "root")

	UpdateOption(context)

	var response map[string]interface{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func TestUpdateModelBillingOptionsSavesValidatedPair(t *testing.T) {
	setupModelBillingOptionTest(t)
	expression := `param("resolution") == "480p" ? tier("480p", per_request(2.5)) : param("resolution") == "720p" ? tier("720p", per_request(3.5)) : tier("invalid", -1)`
	recorder, response := invokeModelBillingOptionUpdate(t, ModelBillingOptionsUpdateRequest{
		BillingMode: map[string]string{"videos-mini": billing_setting.BillingModeTieredExpr},
		BillingExpr: map[string]string{"videos-mini": expression},
		Options: map[string]string{
			"ModelPrice": `{"legacy-model":1.5}`,
		},
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode("videos-mini"))
	storedExpression, ok := billing_setting.GetBillingExpr("videos-mini")
	require.True(t, ok)
	assert.Equal(t, expression, storedExpression)
	modelPrice, hasModelPrice := ratio_setting.GetModelPrice("legacy-model", false)
	assert.True(t, hasModelPrice)
	assert.Equal(t, 1.5, modelPrice)

	var count int64
	require.NoError(t, model.DB.Model(&model.Option{}).Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
		"ModelPrice",
	}).Count(&count).Error)
	assert.Equal(t, int64(3), count)
}

func TestUpdateModelBillingOptionsRejectsInvalidPairWithoutWriting(t *testing.T) {
	setupModelBillingOptionTest(t)
	recorder, response := invokeModelBillingOptionUpdate(t, ModelBillingOptionsUpdateRequest{
		BillingMode: map[string]string{"videos-mini": billing_setting.BillingModeTieredExpr},
		BillingExpr: map[string]string{"videos-mini": `tier("invalid", -1 + per_request(0))`},
		Options: map[string]string{
			"ModelPrice": `{"videos-mini":1.5}`,
		},
	})

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, false, response["success"])
	var count int64
	require.NoError(t, model.DB.Model(&model.Option{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestUpdateModelBillingOptionsRejectsUnsupportedOrInvalidOptions(t *testing.T) {
	cases := []struct {
		name    string
		options map[string]string
	}{
		{name: "unsupported key", options: map[string]string{"GroupRatio": `{}`}},
		{name: "invalid numeric map", options: map[string]string{"ModelPrice": `{"model":"invalid"}`}},
		{name: "invalid boolean", options: map[string]string{"ExposeRatioEnabled": "not-a-bool"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupModelBillingOptionTest(t)
			recorder, response := invokeModelBillingOptionUpdate(t, ModelBillingOptionsUpdateRequest{
				BillingMode: map[string]string{},
				BillingExpr: map[string]string{},
				Options:     tc.options,
			})

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, false, response["success"])
			var count int64
			require.NoError(t, model.DB.Model(&model.Option{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestUpdateOptionRejectsIndividualBillingKeys(t *testing.T) {
	for _, key := range []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	} {
		t.Run(key, func(t *testing.T) {
			setupModelBillingOptionTest(t)
			recorder, response := invokeGenericOptionUpdate(t, key, `{}`)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, false, response["success"])
			var count int64
			require.NoError(t, model.DB.Model(&model.Option{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildModelCapabilityCatalogMergesConfiguredAndRuntimeModels(t *testing.T) {
	pricings := []model.Pricing{
		{
			ModelName:              "square-exact-model",
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
		{
			ModelName:              "square-runtime-model",
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeGemini},
		},
	}
	capabilities := []*model.ModelCapability{
		{
			ModelName: "square-exact-model",
			Config:    `{"endpoints":{"image-generation":{"capabilities":["image.generate"]}}}`,
		},
		{
			ModelName: "configured-only-model",
			Config:    `{"endpoints":{"openai-video":{"capabilities":["video.text_to_video"]}}}`,
		},
	}

	items := buildModelCapabilityCatalog(pricings, capabilities)

	require.Len(t, items, 3)
	assert.Equal(t, "configured-only-model", items[0].ModelName)
	assert.False(t, items[0].Available)
	assert.JSONEq(t, capabilities[1].Config, items[0].Config)
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, items[0].SupportedEndpointTypes)
	assert.Equal(t, "square-exact-model", items[1].ModelName)
	assert.True(t, items[1].Available)
	assert.JSONEq(t, capabilities[0].Config, items[1].Config)
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeImageGeneration,
	}, items[1].SupportedEndpointTypes)
	assert.Equal(t, "square-runtime-model", items[2].ModelName)
	assert.True(t, items[2].Available)
	assert.Empty(t, items[2].Config)
}

func setupModelCapabilityControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.ModelCapability{}))
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestUpdateModelCapabilityDoesNotCreateModelMetadata(t *testing.T) {
	db := setupModelCapabilityControllerTest(t)
	payload, err := common.Marshal(updateModelCapabilityRequest{
		ModelName: "runtime-only-model",
		Config:    `{"endpoints":{"image-generation":{"capabilities":["image.generate"]}}}`,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/models/capabilities", bytes.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")

	UpdateModelCapability(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var metadataCount int64
	require.NoError(t, db.Model(&model.Model{}).Count(&metadataCount).Error)
	assert.Zero(t, metadataCount)
	var capability model.ModelCapability
	require.NoError(t, db.Where("model_name = ?", "runtime-only-model").First(&capability).Error)
	assert.JSONEq(t, `{"endpoints":{"image-generation":{"capabilities":["image.generate"]}}}`, capability.Config)
}

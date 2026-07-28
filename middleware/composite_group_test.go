package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCompositeGroupDistributeBypassesPhysicalChannelSelection(t *testing.T) {
	originalEnabled := setting.CompositeGroupRoutingEnabled
	setting.CompositeGroupRoutingEnabled = true
	t.Cleanup(func() { setting.CompositeGroupRoutingEnabled = originalEnabled })

	db, err := gorm.Open(sqlite.Open("file:middleware-composite-group?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CompositeGroup{}, &model.CompositeGroupRoute{}))
	group := model.CompositeGroup{
		Name:              "image_stable",
		PublicModel:       "admin-image-model",
		Status:            1,
		UserSelectable:    true,
		GenerationEnabled: true,
	}
	require.NoError(t, model.CreateCompositeGroup(db, &group, []model.CompositeGroupRoute{{
		Operation:     model.CompositeOperationGeneration,
		RouteOrder:    1,
		PhysicalGroup: "fixed_image",
		InternalModel: "gpt-image-2-w",
		Status:        1,
	}}))

	originalDB := model.DB
	model.DB = db
	require.NoError(t, service.RefreshCompositeGroupCache())
	t.Cleanup(func() {
		require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.CompositeGroupRoute{}).Error)
		require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.CompositeGroup{}).Error)
		require.NoError(t, service.RefreshCompositeGroupCache())
		model.DB = originalDB
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "image_stable")
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/images/generations", func(c *gin.Context) {
		assert.True(t, service.HasCompositePolicyContext(c))
		_, hasChannel := common.GetContextKey(c, constant.ContextKeyChannelId)
		assert.False(t, hasChannel)
		assert.Equal(t, "admin-image-model", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(`{"model":"admin-image-model"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestCompositeGroupDistributeRejectsWrongPublicModel(t *testing.T) {
	originalEnabled := setting.CompositeGroupRoutingEnabled
	setting.CompositeGroupRoutingEnabled = true
	t.Cleanup(func() { setting.CompositeGroupRoutingEnabled = originalEnabled })

	db, err := gorm.Open(sqlite.Open("file:middleware-composite-group-wrong-model?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CompositeGroup{}, &model.CompositeGroupRoute{}))
	group := model.CompositeGroup{Name: "image_stable", PublicModel: "admin-image-model", Status: 1, UserSelectable: true, GenerationEnabled: true}
	require.NoError(t, model.CreateCompositeGroup(db, &group, []model.CompositeGroupRoute{{Operation: model.CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "fixed_image", InternalModel: "gpt-image-2-w", Status: 1}}))

	originalDB := model.DB
	model.DB = db
	require.NoError(t, service.RefreshCompositeGroupCache())
	t.Cleanup(func() {
		require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.CompositeGroupRoute{}).Error)
		require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.CompositeGroup{}).Error)
		require.NoError(t, service.RefreshCompositeGroupCache())
		model.DB = originalDB
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "image_stable")
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/images/generations", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(`{"model":"gpt-image-2-w"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

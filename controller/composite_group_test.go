package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCompositeGroupControllerTest(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:composite-controller-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CompositeGroup{}, &model.CompositeGroupRoute{}, &model.Token{}))
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
}

func TestAdminCreateCompositeGroupRequiresAdministratorPublicModel(t *testing.T) {
	setupCompositeGroupControllerTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/composite-groups", bytes.NewBufferString(`{
		"name":"image_stable",
		"generation_enabled":true,
		"routes":[{"operation":"image_generation","route_order":1,"physical_group":"gpt_image_web","internal_model":"gpt-image-2-w","status":1}]
	}`))
	context.Request.Header.Set("Content-Type", "application/json")

	AdminCreateCompositeGroup(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "public model")
	assert.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestAdminListCompositeGroupsReturnsStoredPublicModel(t *testing.T) {
	setupCompositeGroupControllerTest(t)
	group := model.CompositeGroup{Name: "image_stable", PublicModel: "admin-image-model", Status: 0, GenerationEnabled: true}
	require.NoError(t, model.CreateCompositeGroup(model.DB, &group, []model.CompositeGroupRoute{{
		Operation: model.CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "gpt_image_web", InternalModel: "gpt-image-2-w", Status: 1,
	}}))
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/composite-groups", nil)

	AdminListCompositeGroups(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "admin-image-model")
	assert.Contains(t, recorder.Body.String(), `"success":true`)
}

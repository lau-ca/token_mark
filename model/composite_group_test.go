package model

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newCompositeGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:composite-group-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CompositeGroup{}, &CompositeGroupRoute{}, &Token{}))
	return db
}

func TestCreateCompositeGroupPreservesAdministratorPublicModel(t *testing.T) {
	db := newCompositeGroupTestDB(t)
	group := CompositeGroup{
		Name:              "image_stable",
		PublicModel:       "admin-image-model",
		Status:            1,
		UserSelectable:    true,
		GenerationEnabled: true,
	}
	routes := []CompositeGroupRoute{
		{
			Operation:        CompositeOperationGeneration,
			RouteOrder:       1,
			PhysicalGroup:    "gpt_image_web",
			InternalModel:    "gpt-image-2-w",
			RetryCount:       2,
			RetryStatusCodes: "429,500-599",
			Status:           1,
		},
	}

	require.NoError(t, CreateCompositeGroup(db, &group, routes))
	got, err := GetCompositeGroupByID(db, group.Id)
	require.NoError(t, err)
	assert.Equal(t, "admin-image-model", got.PublicModel)
	require.Len(t, got.Routes, 1)
	assert.Equal(t, "gpt-image-2-w", got.Routes[0].InternalModel)
}

func TestReplaceCompositeGroupRoutesIsAtomic(t *testing.T) {
	db := newCompositeGroupTestDB(t)
	group := CompositeGroup{Name: "image_stable", PublicModel: "gpt-image-2", Status: 1, GenerationEnabled: true}
	require.NoError(t, CreateCompositeGroup(db, &group, []CompositeGroupRoute{{
		Operation:     CompositeOperationGeneration,
		RouteOrder:    1,
		PhysicalGroup: "gpt_image_web",
		InternalModel: "gpt-image-2-w",
		Status:        1,
	}}))

	updated := group
	updated.DisplayName = "Stable image"
	require.NoError(t, UpdateCompositeGroup(db, &updated, []CompositeGroupRoute{
		{Operation: CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "gpt_image_web", InternalModel: "gpt-image-2-w", Status: 1},
		{Operation: CompositeOperationGeneration, RouteOrder: 2, PhysicalGroup: "codex_image", InternalModel: "gpt-image-2", RetryCount: 1, Status: 1},
	}))

	got, err := GetCompositeGroupByID(db, group.Id)
	require.NoError(t, err)
	assert.Equal(t, "Stable image", got.DisplayName)
	require.Len(t, got.Routes, 2)
	assert.Equal(t, 2, got.Routes[1].RouteOrder)
}

func TestDeleteCompositeGroupRejectsReferencedToken(t *testing.T) {
	db := newCompositeGroupTestDB(t)
	group := CompositeGroup{Name: "image_stable", PublicModel: "gpt-image-2", Status: 1, GenerationEnabled: true}
	require.NoError(t, CreateCompositeGroup(db, &group, []CompositeGroupRoute{{
		Operation: CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "gpt_image_web", InternalModel: "gpt-image-2-w", Status: 1,
	}}))
	require.NoError(t, db.Create(&Token{UserId: 1, Key: "composite-token", Name: "test", Group: group.Name}).Error)

	err := DeleteCompositeGroup(db, group.Id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token")

	var count int64
	require.NoError(t, db.Model(&CompositeGroup{}).Where("id = ?", group.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

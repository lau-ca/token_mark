package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useModelCapabilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Model{}, &ModelCapability{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
	return db
}

func TestMigrateLegacyModelCapabilitiesSeparatesPlaygroundConfig(t *testing.T) {
	db := useModelCapabilityTestDB(t)
	metadata := Model{
		ModelName: "image-model",
		Endpoints: `{
			"image-generation": {
				"path": "/v1/images/generations",
				"method": "POST",
				"custom": "kept",
				"playground": {"capabilities": ["image.generate"]}
			}
		}`,
	}
	require.NoError(t, db.Create(&metadata).Error)

	require.NoError(t, MigrateLegacyModelCapabilities(db))

	var capability ModelCapability
	require.NoError(t, db.Where("model_name = ?", metadata.ModelName).First(&capability).Error)
	assert.JSONEq(t, `{
		"endpoints": {
			"image-generation": {"capabilities": ["image.generate"]}
		}
	}`, capability.Config)
	require.NoError(t, db.First(&metadata, metadata.Id).Error)
	assert.JSONEq(t, `{
		"image-generation": {
			"path": "/v1/images/generations",
			"method": "POST",
			"custom": "kept"
		}
	}`, metadata.Endpoints)
}

func TestMigrateLegacyModelCapabilitiesPreservesExistingConfigAndIsIdempotent(t *testing.T) {
	db := useModelCapabilityTestDB(t)
	metadata := Model{
		ModelName: "video-model",
		Endpoints: `{
			"openai-video": {"playground": {"capabilities": ["video.text_to_video"]}},
			"image-generation": {"playground": {"capabilities": ["image.generate"]}}
		}`,
	}
	require.NoError(t, db.Create(&metadata).Error)
	require.NoError(t, db.Create(&ModelCapability{
		ModelName: metadata.ModelName,
		Config: `{
			"version": 2,
			"endpoints": {
				"openai-video": {"capabilities": ["video.image_to_video"], "future": true}
			}
		}`,
	}).Error)

	require.NoError(t, MigrateLegacyModelCapabilities(db))
	require.NoError(t, MigrateLegacyModelCapabilities(db))

	var capability ModelCapability
	require.NoError(t, db.Where("model_name = ?", metadata.ModelName).First(&capability).Error)
	assert.JSONEq(t, `{
		"version": 2,
		"endpoints": {
			"openai-video": {"capabilities": ["video.image_to_video"], "future": true},
			"image-generation": {"capabilities": ["image.generate"]}
		}
	}`, capability.Config)
}

func TestMigrateLegacyModelCapabilitiesRemovesCapabilityOnlyMetadataPlaceholder(t *testing.T) {
	db := useModelCapabilityTestDB(t)
	metadata := Model{
		ModelName:    "runtime-only-model",
		Endpoints:    `{"image-generation":{"playground":{"capabilities":["image.generate"]}}}`,
		Status:       1,
		SyncOfficial: 1,
		NameRule:     NameRuleExact,
	}
	require.NoError(t, db.Create(&metadata).Error)

	require.NoError(t, MigrateLegacyModelCapabilities(db))

	var metadataCount int64
	require.NoError(t, db.Model(&Model{}).Where("id = ?", metadata.Id).Count(&metadataCount).Error)
	assert.Zero(t, metadataCount)
	var capability ModelCapability
	require.NoError(t, db.Where("model_name = ?", metadata.ModelName).First(&capability).Error)
	assert.JSONEq(t, `{"endpoints":{"image-generation":{"capabilities":["image.generate"]}}}`, capability.Config)
}

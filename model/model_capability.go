package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ModelCapability struct {
	Id          int    `json:"id"`
	ModelName   string `json:"model_name" gorm:"size:128;not null;uniqueIndex"`
	Config      string `json:"config" gorm:"type:text;not null"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func GetAllModelCapabilities() ([]*ModelCapability, error) {
	var capabilities []*ModelCapability
	err := DB.Order("model_name ASC").Find(&capabilities).Error
	return capabilities, err
}

func GetModelCapabilitiesByNames(modelNames []string) (map[string]*ModelCapability, error) {
	modelNames = normalizeLookupValues(modelNames)
	result := make(map[string]*ModelCapability, len(modelNames))
	if len(modelNames) == 0 {
		return result, nil
	}

	var capabilities []*ModelCapability
	if err := DB.Where("model_name IN ?", modelNames).Find(&capabilities).Error; err != nil {
		return nil, err
	}
	for _, capability := range capabilities {
		result[capability.ModelName] = capability
	}
	return result, nil
}

func UpsertModelCapability(modelName, config string) (*ModelCapability, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, errors.New("model name cannot be empty")
	}
	now := common.GetTimestamp()
	capability := ModelCapability{
		ModelName:   modelName,
		Config:      config,
		CreatedTime: now,
		UpdatedTime: now,
	}
	if err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "model_name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"config":       config,
			"updated_time": now,
		}),
	}).Create(&capability).Error; err != nil {
		return nil, err
	}
	if err := DB.Where("model_name = ?", modelName).First(&capability).Error; err != nil {
		return nil, err
	}
	return &capability, nil
}

func MigrateLegacyModelCapabilities(db *gorm.DB) error {
	var metadata []*Model
	if err := db.Where("endpoints <> ''").Find(&metadata).Error; err != nil {
		return err
	}
	for _, item := range metadata {
		cleanedEndpoints, legacyEndpoints, changed, err := extractLegacyModelCapabilities(item.Endpoints)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to inspect legacy model capability for %s: %v", item.ModelName, err))
			continue
		}
		if !changed {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			var existing ModelCapability
			err := tx.Where("model_name = ?", item.ModelName).First(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			mergedConfig, err := mergeLegacyModelCapabilityConfig(existing.Config, legacyEndpoints)
			if err != nil {
				return err
			}
			now := common.GetTimestamp()
			capability := ModelCapability{
				ModelName:   item.ModelName,
				Config:      mergedConfig,
				CreatedTime: now,
				UpdatedTime: now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "model_name"}},
				DoUpdates: clause.Assignments(map[string]any{
					"config":       mergedConfig,
					"updated_time": now,
				}),
			}).Create(&capability).Error; err != nil {
				return err
			}
			if isLegacyCapabilityOnlyModel(item, cleanedEndpoints) {
				return tx.Delete(&Model{}, item.Id).Error
			}
			return tx.Model(&Model{}).Where("id = ?", item.Id).Update("endpoints", cleanedEndpoints).Error
		}); err != nil {
			return fmt.Errorf("migrate model capability for %s: %w", item.ModelName, err)
		}
	}
	return nil
}

func isLegacyCapabilityOnlyModel(item *Model, cleanedEndpoints string) bool {
	if item == nil || item.NameRule != NameRuleExact || item.Status != 1 || item.SyncOfficial != 1 {
		return false
	}
	if item.Description != "" || item.Icon != "" || item.Tags != "" || item.VendorID != 0 {
		return false
	}
	var endpoints map[string]any
	if err := common.UnmarshalJsonStr(cleanedEndpoints, &endpoints); err != nil {
		return false
	}
	return len(endpoints) == 0
}

func extractLegacyModelCapabilities(raw string) (string, map[string]any, bool, error) {
	var endpoints map[string]any
	if err := common.UnmarshalJsonStr(raw, &endpoints); err != nil {
		return raw, nil, false, nil
	}
	legacyEndpoints := make(map[string]any)
	for endpointName, rawEndpoint := range endpoints {
		endpoint, ok := rawEndpoint.(map[string]any)
		if !ok {
			continue
		}
		playground, exists := endpoint["playground"]
		if !exists {
			continue
		}
		legacyEndpoints[endpointName] = playground
		delete(endpoint, "playground")
		if len(endpoint) == 0 {
			delete(endpoints, endpointName)
		} else {
			endpoints[endpointName] = endpoint
		}
	}
	if len(legacyEndpoints) == 0 {
		return raw, nil, false, nil
	}
	cleaned, err := common.Marshal(endpoints)
	if err != nil {
		return raw, nil, false, err
	}
	return string(cleaned), legacyEndpoints, true, nil
}

func mergeLegacyModelCapabilityConfig(existingRaw string, legacyEndpoints map[string]any) (string, error) {
	config := map[string]any{}
	if strings.TrimSpace(existingRaw) != "" {
		if err := common.UnmarshalJsonStr(existingRaw, &config); err != nil {
			return "", err
		}
	}
	endpoints, ok := config["endpoints"].(map[string]any)
	if !ok {
		endpoints = map[string]any{}
		config["endpoints"] = endpoints
	}
	for endpointName, playground := range legacyEndpoints {
		if _, exists := endpoints[endpointName]; !exists {
			endpoints[endpointName] = playground
		}
	}
	encoded, err := common.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

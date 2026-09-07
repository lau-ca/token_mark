package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	CompositeOperationGeneration = "image_generation"
	CompositeOperationEdit       = "image_edit"
)

type CompositeGroup struct {
	Id                int            `json:"id"`
	Name              string         `json:"name" gorm:"size:64;not null;unique"`
	PublicModel       string         `json:"public_model" gorm:"size:128;not null"`
	DisplayName       string         `json:"display_name" gorm:"size:128"`
	Description       string         `json:"description" gorm:"type:text"`
	Status            int            `json:"status" gorm:"index"`
	UserSelectable    bool           `json:"user_selectable"`
	PricingVisible    bool           `json:"pricing_visible"`
	GenerationEnabled bool           `json:"generation_enabled"`
	EditEnabled       bool           `json:"edit_enabled"`
	CreatedTime       int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`

	Routes []CompositeGroupRoute `json:"routes" gorm:"-"`
}

type CompositeGroupRoute struct {
	Id               int            `json:"id"`
	CompositeGroupId int            `json:"composite_group_id" gorm:"index;not null"`
	Operation        string         `json:"operation" gorm:"size:32;not null;index"`
	RouteOrder       int            `json:"route_order" gorm:"not null"`
	PhysicalGroup    string         `json:"physical_group" gorm:"size:64;not null"`
	InternalModel    string         `json:"internal_model" gorm:"size:128;not null"`
	RetryCount       int            `json:"retry_count"`
	RetryStatusCodes string         `json:"retry_status_codes" gorm:"type:text"`
	Status           int            `json:"status" gorm:"index"`
	CreatedTime      int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime      int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func CreateCompositeGroup(db *gorm.DB, group *CompositeGroup, routes []CompositeGroupRoute) error {
	if db == nil {
		return errors.New("database is nil")
	}
	if group == nil {
		return errors.New("composite group is nil")
	}
	normalizeCompositeGroup(group)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		return createCompositeGroupRoutes(tx, group.Id, routes)
	})
}

func UpdateCompositeGroup(db *gorm.DB, group *CompositeGroup, routes []CompositeGroupRoute) error {
	if db == nil {
		return errors.New("database is nil")
	}
	if group == nil || group.Id == 0 {
		return errors.New("composite group id is required")
	}
	normalizeCompositeGroup(group)
	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&CompositeGroup{}).Where("id = ?", group.Id).Select(
			"name", "public_model", "display_name", "description", "status",
			"user_selectable", "pricing_visible", "generation_enabled", "edit_enabled", "updated_time",
		).Updates(group)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("composite_group_id = ?", group.Id).Delete(&CompositeGroupRoute{}).Error; err != nil {
			return err
		}
		return createCompositeGroupRoutes(tx, group.Id, routes)
	})
}

func GetCompositeGroupByID(db *gorm.DB, id int) (*CompositeGroup, error) {
	if db == nil {
		return nil, errors.New("database is nil")
	}
	var group CompositeGroup
	if err := db.First(&group, id).Error; err != nil {
		return nil, err
	}
	routes, err := getCompositeGroupRoutes(db, []int{id})
	if err != nil {
		return nil, err
	}
	group.Routes = routes[id]
	return &group, nil
}

func GetAllCompositeGroups(db *gorm.DB) ([]CompositeGroup, error) {
	if db == nil {
		return nil, errors.New("database is nil")
	}
	var groups []CompositeGroup
	if err := db.Order("id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.Id)
	}
	routes, err := getCompositeGroupRoutes(db, ids)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		groups[i].Routes = routes[groups[i].Id]
	}
	return groups, nil
}

func UpdateCompositeGroupStatus(db *gorm.DB, id int, status int) error {
	if db == nil {
		return errors.New("database is nil")
	}
	result := db.Model(&CompositeGroup{}).Where("id = ?", id).Updates(map[string]any{
		"status":       status,
		"updated_time": common.GetTimestamp(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeleteCompositeGroup(db *gorm.DB, id int) error {
	if db == nil {
		return errors.New("database is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var group CompositeGroup
		if err := tx.First(&group, id).Error; err != nil {
			return err
		}
		var tokenCount int64
		if err := tx.Model(&Token{}).Where(commonGroupCol+" = ?", group.Name).Count(&tokenCount).Error; err != nil {
			return err
		}
		if tokenCount > 0 {
			return fmt.Errorf("composite group is referenced by %d token(s)", tokenCount)
		}
		if err := tx.Where("composite_group_id = ?", id).Delete(&CompositeGroupRoute{}).Error; err != nil {
			return err
		}
		return tx.Delete(&group).Error
	})
}

func normalizeCompositeGroup(group *CompositeGroup) {
	now := common.GetTimestamp()
	group.Name = strings.TrimSpace(group.Name)
	group.PublicModel = strings.TrimSpace(group.PublicModel)
	group.DisplayName = strings.TrimSpace(group.DisplayName)
	if group.CreatedTime == 0 {
		group.CreatedTime = now
	}
	group.UpdatedTime = now
}

func createCompositeGroupRoutes(db *gorm.DB, groupId int, routes []CompositeGroupRoute) error {
	if len(routes) == 0 {
		return nil
	}
	now := common.GetTimestamp()
	items := make([]CompositeGroupRoute, len(routes))
	copy(items, routes)
	for i := range items {
		items[i].Id = 0
		items[i].CompositeGroupId = groupId
		items[i].Operation = strings.TrimSpace(items[i].Operation)
		items[i].PhysicalGroup = strings.TrimSpace(items[i].PhysicalGroup)
		items[i].InternalModel = strings.TrimSpace(items[i].InternalModel)
		items[i].RetryStatusCodes = strings.TrimSpace(items[i].RetryStatusCodes)
		items[i].CreatedTime = now
		items[i].UpdatedTime = now
	}
	return db.Create(&items).Error
}

func getCompositeGroupRoutes(db *gorm.DB, groupIds []int) (map[int][]CompositeGroupRoute, error) {
	result := make(map[int][]CompositeGroupRoute, len(groupIds))
	if len(groupIds) == 0 {
		return result, nil
	}
	var routes []CompositeGroupRoute
	if err := db.Where("composite_group_id IN ?", groupIds).Find(&routes).Error; err != nil {
		return nil, err
	}
	for _, route := range routes {
		result[route.CompositeGroupId] = append(result[route.CompositeGroupId], route)
	}
	for groupId := range result {
		sort.SliceStable(result[groupId], func(i, j int) bool {
			left := result[groupId][i]
			right := result[groupId][j]
			if left.Operation != right.Operation {
				return left.Operation < right.Operation
			}
			return left.RouteOrder < right.RouteOrder
		})
	}
	return result, nil
}

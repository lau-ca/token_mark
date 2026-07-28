package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const MaxCompositeRouteRetryCount = 10

type CompositeRoutePolicy struct {
	Id               int
	Operation        string
	RouteOrder       int
	PhysicalGroup    string
	InternalModel    string
	RetryCount       int
	RetryStatusCodes string
}

type CompositeGroupPolicy struct {
	Id                int
	Name              string
	PublicModel       string
	DisplayName       string
	Description       string
	Enabled           bool
	UserSelectable    bool
	PricingVisible    bool
	GenerationEnabled bool
	EditEnabled       bool
	routes            map[string][]CompositeRoutePolicy
}

type CompositeSelectableGroup struct {
	Name        string
	DisplayName string
	Description string
	PublicModel string
}

type CompositePolicySnapshot struct {
	byGroup map[string]*CompositeGroupPolicy
}

var compositePolicySnapshot atomic.Pointer[CompositePolicySnapshot]

func init() {
	compositePolicySnapshot.Store(&CompositePolicySnapshot{byGroup: map[string]*CompositeGroupPolicy{}})
}

func (snapshot *CompositePolicySnapshot) Resolve(group string) (*CompositeGroupPolicy, bool) {
	if snapshot == nil {
		return nil, false
	}
	policy, ok := snapshot.byGroup[group]
	return policy, ok
}

func (policy *CompositeGroupPolicy) RoutesFor(operation string) []CompositeRoutePolicy {
	if policy == nil {
		return nil
	}
	routes := policy.routes[operation]
	result := make([]CompositeRoutePolicy, len(routes))
	copy(result, routes)
	return result
}

func InitCompositeGroupCache() error {
	return RefreshCompositeGroupCache()
}

func RefreshCompositeGroupCache() error {
	groups, err := model.GetAllCompositeGroups(model.DB)
	if err != nil {
		return err
	}
	snapshot, err := buildCompositePolicySnapshot(groups)
	if err != nil {
		return err
	}
	compositePolicySnapshot.Store(snapshot)
	return nil
}

func SyncCompositeGroupCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		if err := RefreshCompositeGroupCache(); err != nil {
			common.SysError("failed to refresh composite group cache: " + err.Error())
		}
	}
}

func ResolveCompositeGroup(group string) (*CompositeGroupPolicy, bool) {
	return compositePolicySnapshot.Load().Resolve(group)
}

func ListSelectableCompositeGroups(_ string) map[string]CompositeSelectableGroup {
	result := make(map[string]CompositeSelectableGroup)
	snapshot := compositePolicySnapshot.Load()
	if snapshot == nil {
		return result
	}
	for name, policy := range snapshot.byGroup {
		if !policy.Enabled || !policy.UserSelectable {
			continue
		}
		result[name] = CompositeSelectableGroup{
			Name:        name,
			DisplayName: policy.DisplayName,
			Description: policy.Description,
			PublicModel: policy.PublicModel,
		}
	}
	return result
}

func ValidateCompositeGroup(group model.CompositeGroup, routes []model.CompositeGroupRoute) error {
	if err := validateCompositeGroupStructure(group, routes); err != nil {
		return err
	}
	if ratio_setting.ContainsGroupRatio(group.Name) {
		return fmt.Errorf("composite group name %s conflicts with an existing physical group", group.Name)
	}
	for _, route := range routes {
		if route.Status != 1 {
			continue
		}
		if !ratio_setting.ContainsGroupRatio(route.PhysicalGroup) {
			return fmt.Errorf("physical group %s does not exist", route.PhysicalGroup)
		}
		if !hasCompositeModelBilling(route.InternalModel) {
			return fmt.Errorf("internal model %s has no billing configuration", route.InternalModel)
		}
		requestPath := "/v1/images/generations"
		if route.Operation == model.CompositeOperationEdit {
			requestPath = "/v1/images/edits"
		}
		channel, err := model.GetRandomSatisfiedChannel(route.PhysicalGroup, route.InternalModel, 0, requestPath)
		if err != nil {
			return fmt.Errorf("validate route %s/%s: %w", route.PhysicalGroup, route.InternalModel, err)
		}
		if channel == nil {
			return fmt.Errorf("physical group %s has no enabled channel for model %s", route.PhysicalGroup, route.InternalModel)
		}
	}
	return nil
}

func ShouldRetryCompositeStatus(expression string, status int) bool {
	if strings.TrimSpace(expression) == "" {
		return operation_setting.ShouldRetryByStatusCode(status)
	}
	ranges, err := operation_setting.ParseHTTPStatusCodeRanges(expression)
	if err != nil || status < 100 || status > 599 {
		return false
	}
	for _, item := range ranges {
		if status >= item.Start && status <= item.End {
			return true
		}
	}
	return false
}

func buildCompositePolicySnapshot(groups []model.CompositeGroup) (*CompositePolicySnapshot, error) {
	snapshot := &CompositePolicySnapshot{byGroup: make(map[string]*CompositeGroupPolicy)}
	for _, group := range groups {
		if group.Status != 1 {
			continue
		}
		if err := validateCompositeGroupStructure(group, group.Routes); err != nil {
			return nil, fmt.Errorf("invalid composite group %s: %w", group.Name, err)
		}
		policy := &CompositeGroupPolicy{
			Id:                group.Id,
			Name:              group.Name,
			PublicModel:       group.PublicModel,
			DisplayName:       group.DisplayName,
			Description:       group.Description,
			Enabled:           true,
			UserSelectable:    group.UserSelectable,
			PricingVisible:    group.PricingVisible,
			GenerationEnabled: group.GenerationEnabled,
			EditEnabled:       group.EditEnabled,
			routes:            make(map[string][]CompositeRoutePolicy),
		}
		for _, route := range group.Routes {
			if route.Status != 1 {
				continue
			}
			policy.routes[route.Operation] = append(policy.routes[route.Operation], CompositeRoutePolicy{
				Id:               route.Id,
				Operation:        route.Operation,
				RouteOrder:       route.RouteOrder,
				PhysicalGroup:    route.PhysicalGroup,
				InternalModel:    route.InternalModel,
				RetryCount:       route.RetryCount,
				RetryStatusCodes: route.RetryStatusCodes,
			})
		}
		for operation := range policy.routes {
			sort.SliceStable(policy.routes[operation], func(i, j int) bool {
				return policy.routes[operation][i].RouteOrder < policy.routes[operation][j].RouteOrder
			})
		}
		snapshot.byGroup[group.Name] = policy
	}
	return snapshot, nil
}

func validateCompositeGroupStructure(group model.CompositeGroup, routes []model.CompositeGroupRoute) error {
	group.Name = strings.TrimSpace(group.Name)
	group.PublicModel = strings.TrimSpace(group.PublicModel)
	if group.Name == "" {
		return errors.New("composite group name is required")
	}
	if group.PublicModel == "" {
		return errors.New("public model is required")
	}
	if !group.GenerationEnabled && !group.EditEnabled {
		return errors.New("at least one image operation must be enabled")
	}
	enabledCounts := map[string]int{}
	orders := map[string]map[int]struct{}{}
	for _, route := range routes {
		if route.Status != 1 {
			continue
		}
		if route.Operation != model.CompositeOperationGeneration && route.Operation != model.CompositeOperationEdit {
			return fmt.Errorf("unsupported operation %s", route.Operation)
		}
		if route.RouteOrder <= 0 {
			return fmt.Errorf("route order must be positive for %s", route.Operation)
		}
		if strings.TrimSpace(route.PhysicalGroup) == "" {
			return fmt.Errorf("physical group is required for %s route %d", route.Operation, route.RouteOrder)
		}
		if strings.TrimSpace(route.InternalModel) == "" {
			return fmt.Errorf("internal model is required for %s route %d", route.Operation, route.RouteOrder)
		}
		if route.RetryCount < 0 || route.RetryCount > MaxCompositeRouteRetryCount {
			return fmt.Errorf("retry count must be between 0 and %d", MaxCompositeRouteRetryCount)
		}
		if _, err := operation_setting.ParseHTTPStatusCodeRanges(route.RetryStatusCodes); err != nil {
			return err
		}
		if orders[route.Operation] == nil {
			orders[route.Operation] = map[int]struct{}{}
		}
		if _, exists := orders[route.Operation][route.RouteOrder]; exists {
			return fmt.Errorf("duplicate route order %d for %s", route.RouteOrder, route.Operation)
		}
		orders[route.Operation][route.RouteOrder] = struct{}{}
		enabledCounts[route.Operation]++
	}
	if group.GenerationEnabled && enabledCounts[model.CompositeOperationGeneration] == 0 {
		return fmt.Errorf("%s requires at least one enabled route", model.CompositeOperationGeneration)
	}
	if group.EditEnabled && enabledCounts[model.CompositeOperationEdit] == 0 {
		return fmt.Errorf("%s requires at least one enabled route", model.CompositeOperationEdit)
	}
	return nil
}

func hasCompositeModelBilling(modelName string) bool {
	if _, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return true
	}
	if _, ok, _ := ratio_setting.GetModelRatio(modelName); ok {
		return true
	}
	mode, expression, ok := billing_setting.GetModelBillingConfig(modelName)
	return ok && mode == billing_setting.BillingModeTieredExpr && strings.TrimSpace(expression) != ""
}

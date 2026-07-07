package model

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelPricing struct {
	ChannelID  int      `json:"channel_id" gorm:"primaryKey;column:channel_id"`
	Ratio      *float64 `json:"ratio" gorm:"column:ratio"`
	Exchange   *float64 `json:"exchange" gorm:"column:exchange"`
	Margin     *float64 `json:"margin" gorm:"column:margin"`
	Commission *float64 `json:"commission" gorm:"column:commission"`
	Discount   *float64 `json:"discount" gorm:"column:discount"`
}

func (ChannelPricing) TableName() string {
	return "channel_pricing"
}

type ChannelPricingGroupView struct {
	Group string  `json:"group"`
	Ratio float64 `json:"ratio"`
}

type ChannelPricingView struct {
	ChannelID           int                       `json:"channel_id"`
	Ratio               *float64                  `json:"ratio"`
	Exchange            *float64                  `json:"exchange"`
	Margin              *float64                  `json:"margin"`
	Commission          *float64                  `json:"commission"`
	Discount            *float64                  `json:"discount"`
	CostRatio           *float64                  `json:"cost_ratio"`
	EffectiveGroup      string                    `json:"effective_group"`
	EffectiveGroupRatio *float64                  `json:"effective_group_ratio"`
	SaleExchange        float64                   `json:"sale_exchange"`
	NetRevenueRatio     *float64                  `json:"net_revenue_ratio"`
	ActualMargin        *float64                  `json:"actual_margin"`
	SuggestedRatio      *float64                  `json:"suggested_ratio"`
	Missing             []string                  `json:"missing"`
	GroupRatios         []ChannelPricingGroupView `json:"group_ratios"`
}

func (view *ChannelPricingView) ToChannelPricing(channelID int) *ChannelPricing {
	if view == nil {
		return nil
	}
	return &ChannelPricing{
		ChannelID:  channelID,
		Ratio:      view.Ratio,
		Exchange:   view.Exchange,
		Margin:     view.Margin,
		Commission: view.Commission,
		Discount:   view.Discount,
	}
}

func sanitizeOptionalRatio(name string, value *float64, allowZero bool) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) {
		return fmt.Errorf("%s must be finite", name)
	}
	if allowZero {
		if *value < 0 {
			return fmt.Errorf("%s must be not less than 0", name)
		}
		return nil
	}
	if *value <= 0 {
		return fmt.Errorf("%s must be greater than 0", name)
	}
	return nil
}

func ValidateChannelPricing(pricing *ChannelPricing) error {
	if pricing == nil {
		return nil
	}
	if err := sanitizeOptionalRatio("ratio", pricing.Ratio, true); err != nil {
		return err
	}
	if err := sanitizeOptionalRatio("exchange", pricing.Exchange, false); err != nil {
		return err
	}
	if err := sanitizeOptionalRatio("margin", pricing.Margin, true); err != nil {
		return err
	}
	if pricing.Margin != nil && *pricing.Margin >= 1 {
		return fmt.Errorf("margin must be less than 1")
	}
	if err := sanitizeOptionalRatio("commission", pricing.Commission, true); err != nil {
		return err
	}
	if pricing.Commission != nil && *pricing.Commission >= 1 {
		return fmt.Errorf("commission must be less than 1")
	}
	if err := sanitizeOptionalRatio("discount", pricing.Discount, false); err != nil {
		return err
	}
	if pricing.Discount != nil && *pricing.Discount > 1 {
		return fmt.Errorf("discount must be not greater than 1")
	}
	if pricing.Commission != nil && pricing.Discount != nil && *pricing.Commission >= *pricing.Discount {
		return fmt.Errorf("commission must be less than discount")
	}
	return nil
}

func SaveChannelPricingTx(tx *gorm.DB, pricing *ChannelPricing) error {
	if pricing == nil {
		return nil
	}
	if err := ValidateChannelPricing(pricing); err != nil {
		return err
	}
	if pricing.Ratio == nil && pricing.Exchange == nil && pricing.Margin == nil && pricing.Commission == nil && pricing.Discount == nil {
		return tx.Where("channel_id = ?", pricing.ChannelID).Delete(&ChannelPricing{}).Error
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}},
		UpdateAll: true,
	}).Create(pricing).Error
}

func SaveChannelPricing(pricing *ChannelPricing) error {
	return SaveChannelPricingTx(DB, pricing)
}

func DeleteChannelPricing(channelID int) error {
	return DB.Where("channel_id = ?", channelID).Delete(&ChannelPricing{}).Error
}

func GetChannelPricingMap(channelIDs []int) (map[int]*ChannelPricing, error) {
	result := make(map[int]*ChannelPricing, len(channelIDs))
	if len(channelIDs) == 0 {
		return result, nil
	}
	var pricings []*ChannelPricing
	if err := DB.Where("channel_id IN ?", channelIDs).Find(&pricings).Error; err != nil {
		return nil, err
	}
	for _, pricing := range pricings {
		result[pricing.ChannelID] = pricing
	}
	return result, nil
}

func AttachChannelPricing(channels []*Channel, selectedGroup string) error {
	channelIDs := make([]int, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Id == 0 {
			continue
		}
		channelIDs = append(channelIDs, channel.Id)
	}
	pricingMap, err := GetChannelPricingMap(channelIDs)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		channel.Pricing = BuildChannelPricingView(channel, pricingMap[channel.Id], selectedGroup)
	}
	return nil
}

func BuildChannelPricingView(channel *Channel, pricing *ChannelPricing, selectedGroup string) *ChannelPricingView {
	view := &ChannelPricingView{
		ChannelID:    channel.Id,
		SaleExchange: 1,
		Missing:      make([]string, 0, 4),
	}
	if pricing != nil {
		view.Ratio = pricing.Ratio
		view.Exchange = pricing.Exchange
		view.Margin = pricing.Margin
		view.Commission = pricing.Commission
		view.Discount = pricing.Discount
	}

	view.GroupRatios = getChannelPricingGroupViews(channel.Group, selectedGroup)
	effectiveGroup := getLowestGroupRatio(view.GroupRatios)
	if effectiveGroup != nil {
		view.EffectiveGroup = effectiveGroup.Group
		view.EffectiveGroupRatio = &effectiveGroup.Ratio
	}

	if view.Ratio == nil {
		view.Missing = append(view.Missing, "ratio")
	}
	if view.Exchange == nil {
		view.Missing = append(view.Missing, "exchange")
	}
	if view.Margin == nil {
		view.Missing = append(view.Missing, "margin")
	}
	if view.EffectiveGroupRatio == nil || *view.EffectiveGroupRatio <= 0 {
		view.Missing = append(view.Missing, "group_ratio")
	}

	commission := 0.0
	if view.Commission != nil {
		commission = *view.Commission
	}
	discount := 1.0
	if view.Discount != nil {
		discount = *view.Discount
	}
	netMarketingFactor := discount - commission

	if view.Ratio != nil && view.Exchange != nil {
		costRatio := *view.Ratio * *view.Exchange
		view.CostRatio = &costRatio
		if view.EffectiveGroupRatio != nil && *view.EffectiveGroupRatio > 0 && netMarketingFactor > 0 {
			netRevenueRatio := *view.EffectiveGroupRatio * view.SaleExchange * netMarketingFactor
			if netRevenueRatio > 0 {
				view.NetRevenueRatio = &netRevenueRatio
				actualMargin := (netRevenueRatio - costRatio) / netRevenueRatio
				view.ActualMargin = &actualMargin
			}
		}
	}

	if view.CostRatio != nil && view.Margin != nil && *view.Margin < 1 {
		denominator := view.SaleExchange * netMarketingFactor * (1 - *view.Margin)
		if denominator > 0 {
			suggestedRatio := *view.CostRatio / denominator
			view.SuggestedRatio = &suggestedRatio
		}
	}

	return view
}

func getChannelPricingGroupViews(channelGroups string, selectedGroup string) []ChannelPricingGroupView {
	groups := splitChannelGroups(channelGroups)
	if selectedGroup != "" {
		for _, group := range groups {
			if group == selectedGroup {
				groups = []string{selectedGroup}
				break
			}
		}
	}
	views := make([]ChannelPricingGroupView, 0, len(groups))
	for _, group := range groups {
		views = append(views, ChannelPricingGroupView{
			Group: group,
			Ratio: ratio_setting.GetGroupRatio(group),
		})
	}

	return views
}

func splitChannelGroups(channelGroups string) []string {
	groups := make([]string, 0)
	seen := map[string]struct{}{}
	for _, group := range strings.Split(channelGroups, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		return []string{"default"}
	}
	return groups
}

func getLowestGroupRatio(groups []ChannelPricingGroupView) *ChannelPricingGroupView {
	if len(groups) == 0 {
		return nil
	}
	lowest := groups[0]
	for _, group := range groups[1:] {
		if group.Ratio < lowest.Ratio {
			lowest = group
		}
	}
	return &lowest
}

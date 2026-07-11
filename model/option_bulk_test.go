package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareOptionBulkTest(t *testing.T) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	saved := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = saved
		common.OptionMapRWMutex.Unlock()
	})
}

func TestUpdateOptionsBulkAppliesBillingPairTogether(t *testing.T) {
	prepareOptionBulkTest(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Delete(&Option{}).Error)

	saved := billing_setting.GetConfigCopy()
	t.Cleanup(func() {
		billing_setting.ReplaceConfig(saved.BillingMode, saved.BillingExpr)
	})

	modeJSON := `{"videos-mini":"tiered_expr"}`
	expressionJSON := `{"videos-mini":"tier(\"480p\", per_request(2.5))"}`
	require.NoError(t, UpdateOptionsBulk(map[string]string{
		billing_setting.BillingModeOptionKey: modeJSON,
		billing_setting.BillingExprOptionKey: expressionJSON,
	}))

	var options []Option
	require.NoError(t, DB.Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Order("key").Find(&options).Error)
	require.Len(t, options, 2)
	assert.Equal(t, "tiered_expr", billing_setting.GetBillingMode("videos-mini"))
	expression, ok := billing_setting.GetBillingExpr("videos-mini")
	require.True(t, ok)
	assert.Equal(t, `tier("480p", per_request(2.5))`, expression)
}

func TestLoadOptionsFromDatabaseAppliesBillingPairTogether(t *testing.T) {
	prepareOptionBulkTest(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Delete(&Option{}).Error)

	saved := billing_setting.GetConfigCopy()
	t.Cleanup(func() {
		billing_setting.ReplaceConfig(saved.BillingMode, saved.BillingExpr)
	})
	billing_setting.ReplaceConfig(
		map[string]string{"videos-mini": billing_setting.BillingModeRatio},
		map[string]string{"videos-mini": `tier("old", per_request(1))`},
	)
	require.NoError(t, DB.Create(&[]Option{
		{Key: billing_setting.BillingModeOptionKey, Value: `{"videos-mini":"tiered_expr"}`},
		{Key: billing_setting.BillingExprOptionKey, Value: `{"videos-mini":"tier(\"new\", per_request(2.5))"}`},
	}).Error)

	loadOptionsFromDatabase()

	mode, expression, hasExpression := billing_setting.GetModelBillingConfig("videos-mini")
	assert.Equal(t, billing_setting.BillingModeTieredExpr, mode)
	assert.True(t, hasExpression)
	assert.Equal(t, `tier("new", per_request(2.5))`, expression)
}

func TestLoadOptionsFromDatabaseKeepsCurrentBillingPairWhenStoredPairIsIncomplete(t *testing.T) {
	prepareOptionBulkTest(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Delete(&Option{}).Error)

	saved := billing_setting.GetConfigCopy()
	t.Cleanup(func() {
		billing_setting.ReplaceConfig(saved.BillingMode, saved.BillingExpr)
	})
	billing_setting.ReplaceConfig(
		map[string]string{"videos-mini": billing_setting.BillingModeTieredExpr},
		map[string]string{"videos-mini": `tier("current", per_request(2.5))`},
	)
	require.NoError(t, DB.Create(&Option{
		Key:   billing_setting.BillingModeOptionKey,
		Value: `{"videos-mini":"ratio"}`,
	}).Error)

	loadOptionsFromDatabase()

	mode, expression, hasExpression := billing_setting.GetModelBillingConfig("videos-mini")
	assert.Equal(t, billing_setting.BillingModeTieredExpr, mode)
	assert.True(t, hasExpression)
	assert.Equal(t, `tier("current", per_request(2.5))`, expression)
}

func TestLoadOptionsFromDatabaseKeepsCurrentBillingPairWhenStoredPairIsMalformed(t *testing.T) {
	prepareOptionBulkTest(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Delete(&Option{}).Error)

	saved := billing_setting.GetConfigCopy()
	t.Cleanup(func() {
		billing_setting.ReplaceConfig(saved.BillingMode, saved.BillingExpr)
	})
	billing_setting.ReplaceConfig(
		map[string]string{"videos-mini": billing_setting.BillingModeTieredExpr},
		map[string]string{"videos-mini": `tier("current", per_request(2.5))`},
	)
	require.NoError(t, DB.Create(&[]Option{
		{Key: billing_setting.BillingModeOptionKey, Value: `{invalid`},
		{Key: billing_setting.BillingExprOptionKey, Value: `{"videos-mini":"tier(\"new\", per_request(3.5))"}`},
	}).Error)

	loadOptionsFromDatabase()

	mode, expression, hasExpression := billing_setting.GetModelBillingConfig("videos-mini")
	assert.Equal(t, billing_setting.BillingModeTieredExpr, mode)
	assert.True(t, hasExpression)
	assert.Equal(t, `tier("current", per_request(2.5))`, expression)
}

func TestUpdateOptionsBulkRejectsInvalidBillingPairBeforeWrite(t *testing.T) {
	prepareOptionBulkTest(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Delete(&Option{}).Error)

	err := UpdateOptionsBulk(map[string]string{
		billing_setting.BillingModeOptionKey: `{"videos-mini":"tiered_expr"}`,
		billing_setting.BillingExprOptionKey: `{invalid`,
	})
	require.Error(t, err)

	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestUpdateOptionsBulkRejectsIncompleteBillingPairBeforeWrite(t *testing.T) {
	prepareOptionBulkTest(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Delete(&Option{}).Error)

	err := UpdateOptionsBulk(map[string]string{
		billing_setting.BillingModeOptionKey: `{"videos-mini":"tiered_expr"}`,
	})
	require.Error(t, err)

	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key IN ?", []string{
		billing_setting.BillingModeOptionKey,
		billing_setting.BillingExprOptionKey,
	}).Count(&count).Error)
	assert.Zero(t, count)
}

package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testVideosMiniExpr     = `param("resolution") == "480p" ? tier("480p", per_request(2.5)) : param("resolution") == "720p" ? tier("720p", per_request(3.5)) : tier("invalid", -1)`
	testVideosFastExpr     = `param("resolution") == "480p" ? tier("480p", per_request(4)) : param("resolution") == "720p" ? tier("720p", per_request(6)) : tier("invalid", -1)`
	testVideosStandardExpr = `param("resolution") == "480p" ? tier("480p", per_request(5.5)) : param("resolution") == "720p" ? tier("720p", per_request(8)) : param("resolution") == "1080p" ? tier("1080p", per_request(0.9) * param("duration")) : param("resolution") == "4k" ? tier("4k", per_request(2) * param("duration")) : tier("invalid", -1)`
)

func loadTaskPriceTestConfig(t *testing.T, modes, expressions map[string]string, modelPrices map[string]float64) {
	t.Helper()
	savedBilling := billing_setting.GetConfigCopy()
	t.Cleanup(func() {
		billing_setting.ReplaceConfig(savedBilling.BillingMode, savedBilling.BillingExpr)
		billingexpr.InvalidateCache()
	})
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	savedModelPrices := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrices))
	})

	groupRatioJSON, err := common.Marshal(map[string]float64{
		"Seedance2.0": 1,
		"half":        0.5,
	})
	require.NoError(t, err)
	billing_setting.ReplaceConfig(modes, expressions)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"group_ratio_setting.group_ratio": string(groupRatioJSON),
	}))
	if modelPrices != nil {
		modelPriceJSON, err := common.Marshal(modelPrices)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(modelPriceJSON)))
	}
	billingexpr.InvalidateCache()
}

func newTaskPriceTestContext(group string, request relaycommon.TaskSubmitReq) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("group", group)
	ctx.Set("task_request", request)
	return ctx
}

func newTaskPriceTestInfo(modelName, group string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UserGroup:       group,
		UsingGroup:      group,
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: modelName,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action: constant.TaskActionGenerate,
		},
	}
}

func TestModelPriceHelperTaskTieredPriceMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskPriceTestConfig(t,
		map[string]string{
			relaycommon.SeedanceVideoModelMini:     "tiered_expr",
			relaycommon.SeedanceVideoModelFast:     "tiered_expr",
			relaycommon.SeedanceVideoModelStandard: "tiered_expr",
		},
		map[string]string{
			relaycommon.SeedanceVideoModelMini:     testVideosMiniExpr,
			relaycommon.SeedanceVideoModelFast:     testVideosFastExpr,
			relaycommon.SeedanceVideoModelStandard: testVideosStandardExpr,
		},
		nil,
	)

	cases := []struct {
		name       string
		model      string
		resolution string
		duration   int
		group      string
		price      float64
		tier       string
	}{
		{name: "mini 480p", model: relaycommon.SeedanceVideoModelMini, resolution: "480p", duration: 15, group: "Seedance2.0", price: 2.5, tier: "480p"},
		{name: "mini 720p", model: relaycommon.SeedanceVideoModelMini, resolution: "720p", duration: 15, group: "Seedance2.0", price: 3.5, tier: "720p"},
		{name: "fast 480p", model: relaycommon.SeedanceVideoModelFast, resolution: "480p", duration: 15, group: "Seedance2.0", price: 4, tier: "480p"},
		{name: "fast 720p", model: relaycommon.SeedanceVideoModelFast, resolution: "720p", duration: 15, group: "Seedance2.0", price: 6, tier: "720p"},
		{name: "standard 480p", model: relaycommon.SeedanceVideoModelStandard, resolution: "480p", duration: 15, group: "Seedance2.0", price: 5.5, tier: "480p"},
		{name: "standard 720p", model: relaycommon.SeedanceVideoModelStandard, resolution: "720p", duration: 15, group: "Seedance2.0", price: 8, tier: "720p"},
		{name: "standard 1080p minimum", model: relaycommon.SeedanceVideoModelStandard, resolution: "1080p", duration: 4, group: "Seedance2.0", price: 3.6, tier: "1080p"},
		{name: "standard 1080p maximum", model: relaycommon.SeedanceVideoModelStandard, resolution: "1080p", duration: 15, group: "Seedance2.0", price: 13.5, tier: "1080p"},
		{name: "standard 4k minimum", model: relaycommon.SeedanceVideoModelStandard, resolution: "4k", duration: 4, group: "Seedance2.0", price: 8, tier: "4k"},
		{name: "standard 4k maximum", model: relaycommon.SeedanceVideoModelStandard, resolution: "4k", duration: 15, group: "Seedance2.0", price: 30, tier: "4k"},
		{name: "group ratio applies", model: relaycommon.SeedanceVideoModelMini, resolution: "480p", duration: 15, group: "half", price: 1.25, tier: "480p"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := relaycommon.TaskSubmitReq{Model: tc.model, Resolution: tc.resolution, Duration: tc.duration}
			ctx := newTaskPriceTestContext(tc.group, request)
			info := newTaskPriceTestInfo(tc.model, tc.group)

			priceData, err := ModelPriceHelperTask(ctx, info)
			require.NoError(t, err)
			wantQuota := common.QuotaRound(tc.price * common.QuotaPerUnit)
			assert.Equal(t, wantQuota, priceData.Quota)
			assert.Equal(t, wantQuota, priceData.QuotaToPreConsume)
			assert.True(t, priceData.UsePrice)
			require.NotNil(t, info.TieredBillingSnapshot)
			assert.Equal(t, tc.tier, info.TieredBillingSnapshot.EstimatedTier)
			assert.Equal(t, wantQuota, info.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
		})
	}
}

func TestModelPriceHelperTaskFreezesFirstAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const modelName = "task-freeze-model"
	loadTaskPriceTestConfig(t,
		map[string]string{modelName: "tiered_expr"},
		map[string]string{modelName: `tier("first", per_request(2.5))`},
		nil,
	)

	request := relaycommon.TaskSubmitReq{Model: modelName, Resolution: "480p", Duration: 15}
	ctx := newTaskPriceTestContext("Seedance2.0", request)
	info := newTaskPriceTestInfo(modelName, "Seedance2.0")
	first, err := ModelPriceHelperTask(ctx, info)
	require.NoError(t, err)
	require.NotNil(t, info.TieredBillingSnapshot)
	firstHash := info.TieredBillingSnapshot.ExprHash

	billing_setting.ReplaceConfig(
		map[string]string{modelName: billing_setting.BillingModeTieredExpr},
		map[string]string{modelName: `tier("changed", per_request(9))`},
	)
	billingexpr.InvalidateCache()

	second, err := ModelPriceHelperTask(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, first.Quota, second.Quota)
	assert.Equal(t, firstHash, info.TieredBillingSnapshot.ExprHash)
	assert.Equal(t, "first", info.TieredBillingSnapshot.EstimatedTier)
}

func TestModelPriceHelperTaskRequiresPerRequestOptIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const modelName = "legacy-task-model"
	loadTaskPriceTestConfig(t,
		map[string]string{modelName: "tiered_expr"},
		map[string]string{modelName: `tier("tokens", p * 2 + c * 4)`},
		map[string]float64{modelName: 3},
	)

	ctx := newTaskPriceTestContext("Seedance2.0", relaycommon.TaskSubmitReq{Model: modelName})
	info := newTaskPriceTestInfo(modelName, "Seedance2.0")
	priceData, err := ModelPriceHelperTask(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, common.QuotaFromFloat(3*common.QuotaPerUnit), priceData.Quota)
	assert.Nil(t, info.TieredBillingSnapshot)
}

func TestModelPriceHelperTaskRejectsRemixAndInvalidResults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		expr   string
		action string
	}{
		{name: "remix", expr: `tier("base", per_request(2.5))`, action: constant.TaskActionRemix},
		{name: "negative", expr: `tier("invalid", -1 + per_request(0))`, action: constant.TaskActionGenerate},
		{name: "nan", expr: `tier("invalid", per_request(0) / 0)`, action: constant.TaskActionGenerate},
		{name: "infinite", expr: `tier("invalid", per_request(1) / 0)`, action: constant.TaskActionGenerate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modelName := "invalid-" + tc.name
			loadTaskPriceTestConfig(t,
				map[string]string{modelName: "tiered_expr"},
				map[string]string{modelName: tc.expr},
				nil,
			)
			ctx := newTaskPriceTestContext("Seedance2.0", relaycommon.TaskSubmitReq{Model: modelName, Resolution: "480p", Duration: 15})
			info := newTaskPriceTestInfo(modelName, "Seedance2.0")
			info.Action = tc.action

			_, err := ModelPriceHelperTask(ctx, info)
			require.Error(t, err)
			assert.Nil(t, info.TieredBillingSnapshot)
			assert.Zero(t, info.PriceData.Quota)
		})
	}
}

func TestModelPriceHelperTaskAuditsQuotaSaturation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const modelName = "oversized-task-model"
	loadTaskPriceTestConfig(t,
		map[string]string{modelName: "tiered_expr"},
		map[string]string{modelName: `tier("oversized", per_request(1e20))`},
		nil,
	)

	ctx := newTaskPriceTestContext("Seedance2.0", relaycommon.TaskSubmitReq{Model: modelName, Resolution: "480p", Duration: 15})
	info := newTaskPriceTestInfo(modelName, "Seedance2.0")
	priceData, err := ModelPriceHelperTask(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, common.MaxQuota, priceData.Quota)
	require.NotNil(t, info.QuotaClamp)
	assert.Equal(t, common.QuotaClampOverflow, info.QuotaClamp.Kind)
}

package controller

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTaskBillingContextFreezesExpressionData(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "videos-standard",
		PriceData: types.PriceData{
			UsePrice: true,
			Quota:    common.QuotaRound(13.5 * common.QuotaPerUnit),
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:              "tiered_expr",
			ModelName:                "videos-standard",
			ExprString:               `tier("1080p", per_request(0.9) * param("duration"))`,
			ExprHash:                 "original-hash",
			GroupRatio:               1,
			EstimatedQuotaAfterGroup: common.QuotaRound(13.5 * common.QuotaPerUnit),
			EstimatedTier:            "1080p",
		},
	}
	info.PriceData.AddOtherRatio("ignored", 2)
	request := relaycommon.TaskSubmitReq{
		Model:           "videos-standard",
		Resolution:      "1080p",
		Duration:        15,
		Prompt:          "sensitive prompt",
		ReferenceImages: []string{"https://example.com/private.png"},
	}

	context := buildTaskBillingContext(info, request)
	require.NotNil(t, context)
	assert.True(t, context.PerCallBilling)
	assert.Equal(t, "videos-standard", context.OriginModelName)
	assert.Equal(t, "1080p", context.Resolution)
	assert.Equal(t, 15, context.Duration)
	require.NotNil(t, context.TieredBillingSnapshot)
	assert.NotSame(t, info.TieredBillingSnapshot, context.TieredBillingSnapshot)
	assert.Equal(t, "original-hash", context.TieredBillingSnapshot.ExprHash)

	info.TieredBillingSnapshot.ExprHash = "changed-hash"
	info.PriceData.AddOtherRatio("ignored", 3)
	assert.Equal(t, "original-hash", context.TieredBillingSnapshot.ExprHash)
	assert.Equal(t, map[string]float64{"ignored": 2}, context.OtherRatios)

	serialized, err := common.Marshal(context)
	require.NoError(t, err)
	assert.NotContains(t, strings.ToLower(string(serialized)), "sensitive prompt")
	assert.NotContains(t, string(serialized), "private.png")
	assert.NotContains(t, strings.ToLower(string(serialized)), "headers")
}

package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type taskBillingTestAdaptor struct {
	estimateCalls int
	adjustCalls   int
}

func (a *taskBillingTestAdaptor) EstimateBilling(_ *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	a.estimateCalls++
	return map[string]float64{"seconds": 2}
}

func (a *taskBillingTestAdaptor) AdjustBillingOnSubmit(_ *relaycommon.RelayInfo, _ []byte) map[string]float64 {
	a.adjustCalls++
	return map[string]float64{"seconds": 3}
}

func TestTaskBillingExpressionSkipsAdaptorHooks(t *testing.T) {
	savedPatches := append([]string(nil), constant.TaskPricePatches...)
	constant.TaskPricePatches = nil
	t.Cleanup(func() {
		constant.TaskPricePatches = savedPatches
	})

	adaptor := &taskBillingTestAdaptor{}
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{Quota: 100},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
		},
	}

	applyTaskBillingEstimate(nil, info, adaptor, "videos-standard")
	finalQuota := finalizeTaskBillingOnSubmit(info, adaptor, nil)

	assert.Zero(t, adaptor.estimateCalls)
	assert.Zero(t, adaptor.adjustCalls)
	assert.Equal(t, 100, finalQuota)
	assert.Empty(t, info.PriceData.OtherRatios())
}

func TestTaskBillingLegacyKeepsAdaptorHooks(t *testing.T) {
	savedPatches := append([]string(nil), constant.TaskPricePatches...)
	constant.TaskPricePatches = nil
	t.Cleanup(func() {
		constant.TaskPricePatches = savedPatches
	})

	adaptor := &taskBillingTestAdaptor{}
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{Quota: 100},
	}

	applyTaskBillingEstimate(nil, info, adaptor, "legacy-video-model")
	finalQuota := finalizeTaskBillingOnSubmit(info, adaptor, nil)

	assert.Equal(t, 1, adaptor.estimateCalls)
	assert.Equal(t, 1, adaptor.adjustCalls)
	assert.Equal(t, 300, finalQuota)
	assert.Equal(t, map[string]float64{"seconds": 3}, info.PriceData.OtherRatios())
}

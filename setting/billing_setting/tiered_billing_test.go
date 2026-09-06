package billing_setting

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelBillingConfigReadsOneSnapshot(t *testing.T) {
	saved := GetConfigCopy()
	t.Cleanup(func() {
		ReplaceConfig(saved.BillingMode, saved.BillingExpr)
	})

	const modelName = "snapshot-model"
	const firstExpr = `tier("first", per_request(1))`
	const secondExpr = `tier("second", per_request(2))`
	ReplaceConfig(
		map[string]string{modelName: BillingModeTieredExpr},
		map[string]string{modelName: firstExpr},
	)

	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-start
		for iteration := 0; iteration < 64; iteration++ {
			if iteration%2 == 0 {
				ReplaceConfig(
					map[string]string{modelName: BillingModeTieredExpr},
					map[string]string{modelName: firstExpr},
				)
				continue
			}
			ReplaceConfig(
				map[string]string{modelName: BillingModeRatio},
				map[string]string{modelName: secondExpr},
			)
		}
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		for iteration := 0; iteration < 64; iteration++ {
			mode, expr, hasExpr := GetModelBillingConfig(modelName)
			assert.True(t, hasExpr)
			assert.True(t,
				mode == BillingModeTieredExpr && expr == firstExpr ||
					mode == BillingModeRatio && expr == secondExpr,
			)
		}
	}()
	close(start)
	waitGroup.Wait()
}

func TestValidateModelBillingConfigAcceptsSeedanceExpressions(t *testing.T) {
	modes := map[string]string{
		"videos-mini":     BillingModeTieredExpr,
		"videos-fast":     BillingModeTieredExpr,
		"videos-standard": BillingModeTieredExpr,
	}
	expressions := map[string]string{
		"videos-mini":     `param("resolution") == "480p" ? tier("480p", per_request(2.5)) : param("resolution") == "720p" ? tier("720p", per_request(3.5)) : tier("invalid", -1)`,
		"videos-fast":     `param("resolution") == "480p" ? tier("480p", per_request(4)) : param("resolution") == "720p" ? tier("720p", per_request(6)) : tier("invalid", -1)`,
		"videos-standard": `param("resolution") == "480p" ? tier("480p", per_request(5.5)) : param("resolution") == "720p" ? tier("720p", per_request(8)) : param("resolution") == "1080p" ? tier("1080p", per_request(0.9) * param("duration")) : param("resolution") == "4k" ? tier("4k", per_request(2) * param("duration")) : tier("invalid", -1)`,
	}

	require.NoError(t, ValidateModelBillingConfig(modes, expressions))
}

func TestValidateModelBillingConfigRejectsInvalidActiveConfig(t *testing.T) {
	cases := []struct {
		name        string
		modes       map[string]string
		expressions map[string]string
	}{
		{
			name:        "unknown mode",
			modes:       map[string]string{"model": "task_expr"},
			expressions: map[string]string{},
		},
		{
			name:        "missing expression",
			modes:       map[string]string{"model": BillingModeTieredExpr},
			expressions: map[string]string{},
		},
		{
			name:        "compile error",
			modes:       map[string]string{"model": BillingModeTieredExpr},
			expressions: map[string]string{"model": `tier("base",`},
		},
		{
			name:  "negative Seedance sample",
			modes: map[string]string{"videos-mini": BillingModeTieredExpr},
			expressions: map[string]string{
				"videos-mini": `param("resolution") == "480p" ? tier("480p", per_request(2.5)) : tier("invalid", -1)`,
			},
		},
		{
			name:  "infinite Seedance sample",
			modes: map[string]string{"videos-standard": BillingModeTieredExpr},
			expressions: map[string]string{
				"videos-standard": `tier("invalid", per_request(1) / 0)`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			billingexpr.InvalidateCache()
			require.Error(t, ValidateModelBillingConfig(tc.modes, tc.expressions))
		})
	}
}

func TestValidateModelBillingConfigKeepsLegacyTokenExpressions(t *testing.T) {
	require.NoError(t, ValidateModelBillingConfig(
		map[string]string{"legacy-model": BillingModeTieredExpr},
		map[string]string{"legacy-model": `tier("base", p * 2.5 + c * 10)`},
	))
}

func TestValidateModelBillingConfigTaskTokens(t *testing.T) {
	const modelName = "Doubao-Seedance-2.0"
	valid := `task_tokens(param("has_reference_video") ? tier("video", c * 28) : tier("text", c * 46))`
	invalid := `task_tokens(param("has_reference_video") ? tier("video", -1) : tier("text", c * 46))`

	require.NoError(t, ValidateModelBillingConfig(
		map[string]string{modelName: BillingModeTieredExpr},
		map[string]string{modelName: valid},
	))
	require.Error(t, ValidateModelBillingConfig(
		map[string]string{modelName: BillingModeTieredExpr},
		map[string]string{modelName: invalid},
	))
}

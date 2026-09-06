package billing_setting

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/samber/lo"
)

const (
	BillingModeRatio      = "ratio"
	BillingModeTieredExpr = "tiered_expr"
	BillingModeField      = "billing_mode"
	BillingExprField      = "billing_expr"
	BillingModeOptionKey  = "billing_setting.billing_mode"
	BillingExprOptionKey  = "billing_setting.billing_expr"
)

type BillingSetting struct {
	BillingMode map[string]string `json:"billing_mode"`
	BillingExpr map[string]string `json:"billing_expr"`
}

var billingSetting = BillingSetting{
	BillingMode: make(map[string]string),
	BillingExpr: make(map[string]string),
}

var billingSettingMu sync.RWMutex

// ---------------------------------------------------------------------------
// Read accessors (hot path, must be fast)
// ---------------------------------------------------------------------------

func GetBillingMode(model string) string {
	billingSettingMu.RLock()
	defer billingSettingMu.RUnlock()
	if mode, ok := billingSetting.BillingMode[model]; ok {
		return mode
	}
	return BillingModeRatio
}

func GetBillingExpr(model string) (string, bool) {
	billingSettingMu.RLock()
	defer billingSettingMu.RUnlock()
	expr, ok := billingSetting.BillingExpr[model]
	return expr, ok
}

func GetModelBillingConfig(model string) (string, string, bool) {
	billingSettingMu.RLock()
	defer billingSettingMu.RUnlock()
	mode := BillingModeRatio
	if configuredMode, ok := billingSetting.BillingMode[model]; ok {
		mode = configuredMode
	}
	expr, hasExpr := billingSetting.BillingExpr[model]
	return mode, expr, hasExpr
}

func GetConfigCopy() BillingSetting {
	billingSettingMu.RLock()
	defer billingSettingMu.RUnlock()
	return BillingSetting{
		BillingMode: lo.Assign(billingSetting.BillingMode),
		BillingExpr: lo.Assign(billingSetting.BillingExpr),
	}
}

func GetBillingModeCopy() map[string]string {
	return GetConfigCopy().BillingMode
}

func GetBillingExprCopy() map[string]string {
	return GetConfigCopy().BillingExpr
}

func ParseConfigJSON(modeJSON, exprJSON string) (BillingSetting, error) {
	parsed := BillingSetting{
		BillingMode: make(map[string]string),
		BillingExpr: make(map[string]string),
	}
	if err := common.UnmarshalJsonStr(modeJSON, &parsed.BillingMode); err != nil {
		return BillingSetting{}, fmt.Errorf("parse billing mode: %w", err)
	}
	if err := common.UnmarshalJsonStr(exprJSON, &parsed.BillingExpr); err != nil {
		return BillingSetting{}, fmt.Errorf("parse billing expression: %w", err)
	}
	return parsed, nil
}

func ReplaceConfig(modes, expressions map[string]string) {
	billingSettingMu.Lock()
	defer billingSettingMu.Unlock()
	billingSetting.BillingMode = lo.Assign(modes)
	billingSetting.BillingExpr = lo.Assign(expressions)
}

func GetPricingSyncData(base map[string]any) map[string]any {
	settings := GetConfigCopy()
	extra := make(map[string]any, 2)
	if len(settings.BillingMode) > 0 {
		extra[BillingModeField] = settings.BillingMode
	}
	if len(settings.BillingExpr) > 0 {
		extra[BillingExprField] = settings.BillingExpr
	}
	return lo.Assign(base, extra)
}

// ---------------------------------------------------------------------------
// Smoke test (called externally for validation before save)
// ---------------------------------------------------------------------------

func SmokeTestExpr(exprStr string) error {
	return smokeTestExpr(exprStr)
}

func smokeTestExpr(exprStr string) error {
	vectors := []billingexpr.TokenParams{
		{P: 0, C: 0, Len: 0},
		{P: 1000, C: 1000, Len: 1000},
		{P: 100000, C: 100000, Len: 100000},
		{P: 1000000, C: 1000000, Len: 1000000},
	}
	requests := []billingexpr.RequestInput{
		{},
		{
			Headers: map[string]string{
				"anthropic-beta": "fast-mode-2026-02-01",
			},
			Body: []byte(`{"service_tier":"fast","stream_options":{"include_usage":true},"messages":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`),
		},
	}

	for _, v := range vectors {
		for _, request := range requests {
			result, _, err := billingexpr.RunExprWithRequest(exprStr, v, request)
			if err != nil {
				return fmt.Errorf("vector {p=%g, c=%g}: run failed: %w", v.P, v.C, err)
			}
			if math.IsNaN(result) || math.IsInf(result, 0) || result < 0 {
				return fmt.Errorf("vector {p=%g, c=%g}: invalid result %g", v.P, v.C, result)
			}
		}
	}
	return nil
}

func ValidateModelBillingConfig(modes, expressions map[string]string) error {
	for modelName, mode := range modes {
		if mode != BillingModeRatio && mode != BillingModeTieredExpr {
			return fmt.Errorf("model %s has unsupported billing mode %q", modelName, mode)
		}
	}
	for modelName, exprStr := range expressions {
		if strings.TrimSpace(exprStr) == "" {
			continue
		}
		if _, err := billingexpr.CompileFromCache(exprStr); err != nil {
			return fmt.Errorf("model %s billing expression is invalid: %w", modelName, err)
		}
	}
	for modelName, mode := range modes {
		if mode != BillingModeTieredExpr {
			continue
		}
		exprStr := strings.TrimSpace(expressions[modelName])
		if exprStr == "" {
			return fmt.Errorf("model %s uses tiered_expr but has no billing expression", modelName)
		}
		usedVars := billingexpr.UsedVars(exprStr)
		if usedVars["per_request"] || usedVars["task_tokens"] {
			if err := smokeTestTaskExpr(modelName, exprStr, usedVars["task_tokens"]); err != nil {
				return fmt.Errorf("model %s task billing expression failed validation: %w", modelName, err)
			}
			continue
		}
		if err := smokeTestExpr(exprStr); err != nil {
			return fmt.Errorf("model %s billing expression failed validation: %w", modelName, err)
		}
	}
	return nil
}

func smokeTestTaskExpr(modelName, exprStr string, tokenBased bool) error {
	var resolutions []string
	switch modelName {
	case "videos-mini", "videos-fast":
		resolutions = []string{"480p", "720p"}
	case "videos-standard":
		resolutions = []string{"480p", "720p", "1080p", "4k"}
	default:
		if !tokenBased {
			return nil
		}
		resolutions = []string{"480p", "720p", "1080p", "4k"}
	}
	for _, resolution := range resolutions {
		for _, duration := range []int{4, 15} {
			hasVideoCases := []bool{false}
			if tokenBased {
				hasVideoCases = append(hasVideoCases, true)
			}
			for _, hasReferenceVideo := range hasVideoCases {
				body, err := common.Marshal(map[string]interface{}{
					"model":               modelName,
					"resolution":          resolution,
					"duration":            duration,
					"has_reference_video": hasReferenceVideo,
				})
				if err != nil {
					return err
				}
				params := billingexpr.TokenParams{}
				if tokenBased {
					params.C = 1
				}
				result, _, err := billingexpr.RunExprWithRequest(exprStr, params, billingexpr.RequestInput{Body: body})
				if err != nil {
					return fmt.Errorf("resolution=%s duration=%d has_reference_video=%t: %w", resolution, duration, hasReferenceVideo, err)
				}
				if math.IsNaN(result) || math.IsInf(result, 0) || result < 0 {
					return fmt.Errorf("resolution=%s duration=%d has_reference_video=%t: invalid result %g", resolution, duration, hasReferenceVideo, result)
				}
			}
		}
	}
	return nil
}

# Task Token Expression Billing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let asynchronous video tasks select a per-million-token price through a request-aware expression and settle against upstream `completion_tokens` after success.

**Architecture:** Add an explicit `task_tokens()` expression marker that leaves existing token and `per_request()` expressions unchanged. Normalize reference-video state once, freeze the expression and request values in `TaskBillingContext`, then evaluate the frozen expression with actual completion tokens during polling and settle through `RecalculateTaskQuota`.

**Tech Stack:** Go 1.22+, Gin, expr-lang, JSON-backed task private data, testify.

---

## File Map

- `pkg/billingexpr/{compile.go,run.go,expr.md,billingexpr_test.go}`: define and test the explicit Task-token marker.
- `relay/common/{relay_info.go,relay_utils_test.go}`: normalize reference-video detection.
- `relay/channel/task/doubao/{adaptor.go,adaptor_test.go}`: reuse normalized detection.
- `relay/helper/{task_price.go,task_price_test.go}`: freeze token-expression billing state at submission.
- `setting/billing_setting/{tiered_billing.go,tiered_billing_test.go}`: validate both video-input branches.
- `model/task.go`, `controller/relay.go`: persist normalized request state.
- `service/{task_billing.go,task_polling.go,task_billing_test.go}`: settle against actual completion tokens.

### Task 1: Add the explicit `task_tokens()` marker

**Files:**
- Modify: `pkg/billingexpr/compile.go`
- Modify: `pkg/billingexpr/run.go`
- Modify: `pkg/billingexpr/expr.md`
- Test: `pkg/billingexpr/billingexpr_test.go`

- [ ] **Step 1: Write the failing marker test**

```go
func TestTaskTokensMarker(t *testing.T) {
	expression := `task_tokens(tier("video", c * 46))`
	result, trace, err := billingexpr.RunExpr(expression, billingexpr.TokenParams{C: 100_000})
	require.NoError(t, err)
	assert.Equal(t, 4_600_000.0, result)
	assert.Equal(t, "video", trace.MatchedTier)
	assert.True(t, billingexpr.UsedVars(expression)["task_tokens"])
}
```

- [ ] **Step 2: Verify the test fails before implementation**

Run: `go test ./pkg/billingexpr -run TestTaskTokensMarker -count=1`

Expected: FAIL because `task_tokens` is undefined.

- [ ] **Step 3: Implement the identity marker**

Add beside `perRequest`:

```go
func taskTokens(cost float64) float64 {
	return cost
}
```

Register in both compile-time and runtime environments:

```go
"task_tokens": taskTokens,
```

Document that it explicitly opts an asynchronous Task into final token settlement.

- [ ] **Step 4: Run marker tests**

Run: `go test ./pkg/billingexpr -run 'TestTaskTokensMarker|TestPerRequest' -count=1`

Expected: PASS.

### Task 2: Normalize reference-video detection once

**Files:**
- Modify: `relay/common/relay_info.go`
- Test: `relay/common/relay_utils_test.go`
- Modify: `relay/channel/task/doubao/adaptor.go`
- Test: `relay/channel/task/doubao/adaptor_test.go`

- [ ] **Step 1: Write failing normalization tests**

```go
func TestTaskSubmitReqHasReferenceVideo(t *testing.T) {
	tests := []struct {
		name string
		req  TaskSubmitReq
		want bool
	}{
		{name: "reference videos", req: TaskSubmitReq{ReferenceVideos: []string{"https://example.com/reference.mp4"}}, want: true},
		{name: "native content", req: TaskSubmitReq{Content: []TaskContentItem{{Type: "video_url", VideoURL: &TaskMediaURL{URL: "https://example.com/reference.mp4"}}}}, want: true},
		{name: "metadata content", req: TaskSubmitReq{Metadata: map[string]interface{}{"content": []interface{}{map[string]interface{}{"type": "video_url", "video_url": map[string]interface{}{"url": "https://example.com/reference.mp4"}}}}}, want: true},
		{name: "empty video url", req: TaskSubmitReq{Content: []TaskContentItem{{Type: "video_url", VideoURL: &TaskMediaURL{}}}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.req.HasReferenceVideo())
		})
	}
}
```

- [ ] **Step 2: Verify the test fails before implementation**

Run: `go test ./relay/common -run TestTaskSubmitReqHasReferenceVideo -count=1`

Expected: FAIL because `HasReferenceVideo` does not exist.

- [ ] **Step 3: Implement the shared method**

```go
func (t *TaskSubmitReq) HasReferenceVideo() bool {
	if t == nil {
		return false
	}
	for _, videoURL := range t.ReferenceVideos {
		if strings.TrimSpace(videoURL) != "" {
			return true
		}
	}
	for _, item := range t.Content {
		if item.Type == "video_url" && item.VideoURL != nil && strings.TrimSpace(item.VideoURL.URL) != "" {
			return true
		}
	}
	content, ok := t.Metadata["content"].([]interface{})
	if !ok {
		return false
	}
	for _, item := range content {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["type"] == "video_url" {
			return true
		}
		if _, ok := entry["video_url"]; ok {
			return true
		}
	}
	return false
}
```

Change Doubao billing estimation to `hasVideo := req.HasReferenceVideo()` and remove its duplicate local detection helpers.

- [ ] **Step 4: Run normalized request and Doubao tests**

Run: `go test ./relay/common ./relay/channel/task/doubao -count=1`

Expected: PASS.

### Task 3: Freeze token-expression state at submission

**Files:**
- Modify: `relay/helper/task_price.go`
- Test: `relay/helper/task_price_test.go`
- Modify: `model/task.go`
- Modify: `controller/relay.go`

- [ ] **Step 1: Write the failing submission test**

```go
const testTaskTokenExpr = `task_tokens(param("resolution") == "1080p" ? tier("1080p", c * (param("has_reference_video") ? 31 : 51)) : tier("base", c * (param("has_reference_video") ? 28 : 46)))`

func TestModelPriceHelperTaskFreezesTokenExpressionRequest(t *testing.T) {
	const modelName = "Doubao-Seedance-2.0"
	loadTaskPriceTestConfig(t,
		map[string]string{modelName: billing_setting.BillingModeTieredExpr},
		map[string]string{modelName: testTaskTokenExpr},
		map[string]float64{modelName: 23},
	)
	request := relaycommon.TaskSubmitReq{Model: modelName, Resolution: "1080p", ReferenceVideos: []string{"https://example.com/reference.mp4"}}
	ctx := newTaskPriceTestContext("Seedance2.0", request)
	info := newTaskPriceTestInfo(modelName, "Seedance2.0")
	priceData, err := ModelPriceHelperTask(ctx, info)
	require.NoError(t, err)
	assert.Positive(t, priceData.Quota)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.NotNil(t, info.BillingRequestInput)
	assert.True(t, billingexpr.UsedVars(info.TieredBillingSnapshot.ExprString)["task_tokens"])
	assert.JSONEq(t, `{"model":"Doubao-Seedance-2.0","resolution":"1080p","has_reference_video":true}`, string(info.BillingRequestInput.Body))
}
```

- [ ] **Step 2: Verify the test currently falls back without a snapshot**

Run: `go test ./relay/helper -run TestModelPriceHelperTaskFreezesTokenExpressionRequest -count=1`

Expected: FAIL because only `per_request()` opts into Task expressions.

- [ ] **Step 3: Share normalized Task expression input**

Use one structure for both Task expression modes:

```go
type normalizedTaskBillingRequest struct {
	Model             string `json:"model"`
	Operation         string `json:"operation,omitempty"`
	Resolution        string `json:"resolution,omitempty"`
	Duration          int    `json:"duration,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Sound             *bool  `json:"sound,omitempty"`
	HasReferenceVideo bool   `json:"has_reference_video"`
	HasVoice          *bool  `json:"has_voice,omitempty"`
}
```

Populate `HasReferenceVideo` from `taskRequest.HasReferenceVideo()` for every Task channel while retaining Qianfan metadata overrides.

- [ ] **Step 4: Add the explicit token-expression branch**

```go
usedVars := billingexpr.UsedVars(exprStr)
switch {
case usedVars["task_tokens"]:
	return modelPriceHelperTaskTokens(c, info, exprStr)
case usedVars["per_request"]:
	return modelPriceHelperTaskTiered(c, info, exprStr)
default:
	return ModelPriceHelperPerCall(c, info)
}
```

`modelPriceHelperTaskTokens` must preserve the current `ModelPriceHelperPerCall` reservation, freeze the expression, run it with `C: 0` only to record the request-selected tier, and store `BillingRequestInput`.

- [ ] **Step 5: Persist reference-video state**

Add to `TaskBillingContext`:

```go
HasReferenceVideo bool `json:"has_reference_video,omitempty"`
```

Populate in `buildTaskBillingContext`:

```go
HasReferenceVideo: request.HasReferenceVideo(),
```

- [ ] **Step 6: Run focused submission tests**

Run: `go test ./relay/helper ./controller -run 'Task|BillingContext' -count=1`

Expected: PASS, including existing `per_request()` and legacy fallback tests.

### Task 4: Validate both expression branches on save

**Files:**
- Modify: `setting/billing_setting/tiered_billing.go`
- Test: `setting/billing_setting/tiered_billing_test.go`

- [ ] **Step 1: Write failing validation tests**

```go
func TestValidateModelBillingConfigTaskTokens(t *testing.T) {
	const modelName = "Doubao-Seedance-2.0"
	valid := `task_tokens(param("has_reference_video") ? tier("video", c * 28) : tier("text", c * 46))`
	invalid := `task_tokens(param("has_reference_video") ? tier("video", -1) : tier("text", c * 46))`
	require.NoError(t, ValidateModelBillingConfig(map[string]string{modelName: BillingModeTieredExpr}, map[string]string{modelName: valid}))
	require.Error(t, ValidateModelBillingConfig(map[string]string{modelName: BillingModeTieredExpr}, map[string]string{modelName: invalid}))
}
```

- [ ] **Step 2: Verify the negative branch is currently missed**

Run: `go test ./setting/billing_setting -run TestValidateModelBillingConfigTaskTokens -count=1`

Expected: FAIL because `task_tokens()` has no Task smoke test.

- [ ] **Step 3: Extend Task smoke testing**

```go
usedVars := billingexpr.UsedVars(exprStr)
if usedVars["per_request"] || usedVars["task_tokens"] {
	if err := smokeTestTaskExpr(modelName, exprStr, usedVars["task_tokens"]); err != nil {
		return fmt.Errorf("model %s task billing expression failed validation: %w", modelName, err)
	}
	continue
}
```

For `task_tokens()` expressions, evaluate every supported resolution with `C: 1` and `has_reference_video` set to both false and true. Reject negative, NaN, and infinite results.

- [ ] **Step 4: Run billing-setting tests**

Run: `go test ./setting/billing_setting -count=1`

Expected: PASS.

### Task 5: Settle with actual completion tokens

**Files:**
- Modify: `service/task_billing.go`
- Modify: `service/task_polling.go`
- Test: `service/task_billing_test.go`

- [ ] **Step 1: Write failing six-tier tests**

Use `completionTokens = 100_000`, group ratio `1`, and these expected prices:

```go
tests := []struct {
	resolution string
	hasVideo   bool
	price      float64
}{
	{resolution: "720p", hasVideo: false, price: 46},
	{resolution: "720p", hasVideo: true, price: 28},
	{resolution: "1080p", hasVideo: false, price: 51},
	{resolution: "1080p", hasVideo: true, price: 31},
	{resolution: "4k", hasVideo: false, price: 26},
	{resolution: "4k", hasVideo: true, price: 16},
}
```

Expected quota per case:

```go
wantQuota := common.QuotaRound(float64(completionTokens) * tt.price / 1_000_000 * common.QuotaPerUnit)
```

- [ ] **Step 2: Verify expression tasks are currently skipped**

Run: `go test ./service -run 'TaskTokenExpression|SeedanceExpression' -count=1`

Expected: FAIL because polling retains every expression task's pre-consumed quota.

- [ ] **Step 3: Implement expression settlement**

```go
func RecalculateTaskQuotaByExpression(ctx context.Context, task *model.Task, completionTokens int) bool {
	bc := task.PrivateData.BillingContext
	if bc == nil || bc.TieredBillingSnapshot == nil || completionTokens <= 0 {
		return false
	}
	snapshot := bc.TieredBillingSnapshot
	if !billingexpr.UsedVars(snapshot.ExprString)["task_tokens"] {
		return false
	}
	body, err := common.Marshal(map[string]interface{}{
		"model":               bc.OriginModelName,
		"resolution":          bc.Resolution,
		"duration":            bc.Duration,
		"has_reference_video": bc.HasReferenceVideo,
	})
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("构建任务表达式输入失败 task %s: %s", task.TaskID, err.Error()))
		return true
	}
	result, err := billingexpr.ComputeTieredQuotaWithRequest(snapshot, billingexpr.TokenParams{C: float64(completionTokens)}, billingexpr.RequestInput{Body: body})
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("任务表达式结算失败 task %s: %s", task.TaskID, err.Error()))
		return true
	}
	snapshot.EstimatedTier = result.MatchedTier
	RecalculateTaskQuota(ctx, task, result.ActualQuotaAfterGroup, "表达式 token 重算", result.Clamp)
	return true
}
```

Use the project JSON wrapper and checked quota conversion through `ComputeTieredQuotaWithRequest`; do not add local float-to-int casts.

- [ ] **Step 4: Route polling by expression type**

```go
if RecalculateTaskQuotaByExpression(ctx, task, taskResult.CompletionTokens) {
	return
}
if billingContext != nil && billingContext.TieredBillingSnapshot != nil {
	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 按次表达式计费，跳过差额结算", task.TaskID))
	return
}
```

Keep legacy token settlement using `TotalTokens` after this branch.

- [ ] **Step 5: Add charge, refund, equal-quota, and malformed-expression tests**

Assert that a lower reservation causes a supplementary charge, a higher reservation causes a refund, equal quotas cause no adjustment, and a malformed frozen expression retains the reservation without a negative charge.

- [ ] **Step 6: Run focused service tests**

Run: `go test ./service -run 'TaskTokenExpression|SeedanceExpression|RecalculateTaskQuota' -count=1`

Expected: PASS.

### Task 6: Verify the complete backend path

**Files:**
- Verify all modified Go files.

- [ ] **Step 1: Format modified Go files**

Run:

```bash
gofmt -w pkg/billingexpr/compile.go pkg/billingexpr/run.go pkg/billingexpr/billingexpr_test.go relay/common/relay_info.go relay/common/relay_utils_test.go relay/channel/task/doubao/adaptor.go relay/channel/task/doubao/adaptor_test.go relay/helper/task_price.go relay/helper/task_price_test.go setting/billing_setting/tiered_billing.go setting/billing_setting/tiered_billing_test.go model/task.go controller/relay.go service/task_billing.go service/task_polling.go service/task_billing_test.go
```

Expected: exits successfully.

- [ ] **Step 2: Run focused package tests**

Run: `go test ./pkg/billingexpr ./relay/common ./relay/channel/task/doubao ./relay/helper ./setting/billing_setting ./service ./controller -count=1`

Expected: PASS.

- [ ] **Step 3: Build the root module**

Run: `go build ./...`

Expected: PASS. No `relaykit/` files are modified, so its independent build is not required.

- [ ] **Step 4: Review the exact diff**

Run:

```bash
git diff --check
git diff -- pkg/billingexpr relay/common/relay_info.go relay/common/relay_utils_test.go relay/channel/task/doubao relay/helper/task_price.go relay/helper/task_price_test.go setting/billing_setting model/task.go controller/relay.go service/task_billing.go service/task_polling.go service/task_billing_test.go
```

Expected: no whitespace errors and no unrelated edits from this implementation.

### Task 7: Configure and verify `Doubao-Seedance-2.0`

**Files:**
- No repository files; use the authenticated pricing page after deployment.

- [ ] **Step 1: Deploy through the established master/worker procedure**

Preserve unrelated server configuration and verify health before changing pricing.

- [ ] **Step 2: Change the existing exact model to Expression mode**

Model identifier:

```text
Doubao-Seedance-2.0
```

Expression:

```text
task_tokens(param("resolution") == "4k" ? tier("4k", c * (param("has_reference_video") ? 16 : 26)) : param("resolution") == "1080p" ? tier("1080p", c * (param("has_reference_video") ? 31 : 51)) : tier("480p_720p", c * (param("has_reference_video") ? 28 : 46)))
```

- [ ] **Step 3: Save and reopen the entry**

Expected: expression mode remains active, the expression is preserved exactly, and no duplicate model is created.

- [ ] **Step 4: Verify the six combinations**

Use the same completion-token count and confirm matched prices of 46, 28, 51, 31, 26, and 16.

# Task Seconds/Duration Billing Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make request-aware task billing charge the same duration when clients send either `duration` or `seconds`, while preserving existing `duration` behavior.

**Architecture:** Add one shared duration resolver in `relay/common` and use it at the Seedance validation and tiered task billing boundaries. Keep generic task validation lenient and leave the original task request unchanged so legacy provider adaptors continue receiving the same fields they receive today.

**Tech Stack:** Go 1.22+, Gin, testify `require`/`assert`, billingexpr

---

### Task 1: Add shared task-duration resolution for Seedance

**Files:**
- Modify: `relay/common/relay_utils.go`
- Test: `relay/common/relay_utils_test.go`

- [ ] **Step 1: Write the failing resolver tests**

Add a table test that unmarshals JSON into `TaskSubmitReq`, calls `ResolveTaskDuration`, and covers duration-only, seconds-only, matching fields, conflicting fields, invalid seconds, and missing fields.

```go
func TestResolveTaskDuration(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantDuration int
		wantErr      bool
	}{
		{name: "duration only", body: `{"duration":10}`, wantDuration: 10},
		{name: "seconds only", body: `{"seconds":"5"}`, wantDuration: 5},
		{name: "matching fields", body: `{"duration":7,"seconds":"7"}`, wantDuration: 7},
		{name: "conflicting fields", body: `{"duration":7,"seconds":"5"}`, wantErr: true},
		{name: "invalid seconds", body: `{"seconds":"five"}`, wantErr: true},
		{name: "missing fields", body: `{}`, wantDuration: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request TaskSubmitReq
			require.NoError(t, appcommon.Unmarshal([]byte(tt.body), &request))
			duration, err := ResolveTaskDuration(request)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDuration, duration)
		})
	}
}
```

- [ ] **Step 2: Run the resolver test and verify it fails**

Run: `go test ./relay/common -run TestResolveTaskDuration -count=1`

Expected: build failure because `ResolveTaskDuration` does not exist.

- [ ] **Step 3: Implement the shared resolver and reuse it in validation**

Add:

```go
func ResolveTaskDuration(req TaskSubmitReq) (int, error) {
	if req.durationParseErr != nil {
		return 0, req.durationParseErr
	}
	duration := req.Duration
	hasDuration := req.durationProvided || req.Duration != 0
	if req.Seconds == "" {
		return duration, nil
	}
	seconds, err := strconv.Atoi(req.Seconds)
	if err != nil {
		return 0, fmt.Errorf("seconds must be an integer")
	}
	if hasDuration && seconds != duration {
		return 0, fmt.Errorf("duration and seconds must match when both are provided")
	}
	return seconds, nil
}
```

Call it from `ResolveSeedanceVideoDuration`, preserving the existing four-second default and 4–15 second bounds. Do not tighten `validateTaskDurationBounds`, because existing non-Seedance providers rely on its legacy leniency.

- [ ] **Step 4: Run common relay tests**

Run: `go test ./relay/common -count=1`

Expected: PASS.

### Task 2: Feed normalized duration into tiered task billing

**Files:**
- Modify: `relay/helper/task_price.go`
- Test: `relay/helper/task_price_test.go`

- [ ] **Step 1: Add a failing seconds-based billing case**

Extend the existing task price matrix with a `seconds string` field and add a 1080p, five-second case expecting `$4.50` from the existing `$0.90 × duration` expression.

```go
{
	name:       "standard 1080p seconds compatibility",
	model:      relaycommon.SeedanceVideoModelStandard,
	resolution: "1080p",
	seconds:    "5",
	group:      "Seedance2.0",
	price:      4.5,
	tier:       "1080p",
}
```

Construct the request with both supported fields:

```go
request := relaycommon.TaskSubmitReq{
	Model:      tc.model,
	Resolution: tc.resolution,
	Duration:   tc.duration,
	Seconds:    tc.seconds,
}
```

- [ ] **Step 2: Run the billing test and verify it fails**

Run: `go test ./relay/helper -run TestModelPriceHelperTaskTieredPriceMatrix -count=1`

Expected: the seconds case calculates zero instead of the quota for `$4.50`.

- [ ] **Step 3: Resolve duration before building billing expression input**

Add:

```go
duration, err := relaycommon.ResolveTaskDuration(taskRequest)
if err != nil {
	return types.PriceData{}, fmt.Errorf("resolve normalized task billing duration: %w", err)
}
```

Use `duration` instead of `taskRequest.Duration` in the normalized billing request.

- [ ] **Step 4: Run focused and package tests**

Run:

```bash
go test ./relay/helper -run TestModelPriceHelperTask -count=1
go test ./relay/common ./relay/helper -count=1
```

Expected: PASS.

### Task 3: Format and verify the complete change

**Files:**
- Modify: `relay/common/relay_utils.go`
- Modify: `relay/common/relay_utils_test.go`
- Modify: `relay/helper/task_price.go`
- Modify: `relay/helper/task_price_test.go`

- [ ] **Step 1: Format modified Go files**

Run: `gofmt -w relay/common/relay_utils.go relay/common/relay_utils_test.go relay/helper/task_price.go relay/helper/task_price_test.go`

- [ ] **Step 2: Check the focused diff and whitespace**

Run:

```bash
git diff --check -- relay/common/relay_utils.go relay/common/relay_utils_test.go relay/helper/task_price.go relay/helper/task_price_test.go
git diff -- relay/common/relay_utils.go relay/common/relay_utils_test.go relay/helper/task_price.go relay/helper/task_price_test.go
```

Expected: no whitespace errors and no unrelated changes.

- [ ] **Step 3: Run final regression tests**

Run: `go test ./relay/common ./relay/helper -count=1`

Expected: PASS.

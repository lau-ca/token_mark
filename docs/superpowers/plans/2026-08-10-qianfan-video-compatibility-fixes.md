# Qianfan Video Compatibility Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the three public Kling models follow the documented Qianfan request contracts, return complete OpenAI-compatible task responses, and expose the native Qianfan route through the API Nginx entrypoint without changing other providers.

**Architecture:** Keep the existing Baidu V2 channel and Qianfan task adaptor. Normalize and validate only Qianfan compatibility requests, preserve native provider request bodies, complete the Qianfan response converter, and add a Baidu-V2-only Range forwarding branch in the shared content proxy. Add an explicit Nginx location for `/qianfan/` instead of widening existing routing rules.

**Tech Stack:** Go 1.22+, Gin, testify, Nginx, existing task relay and billing-expression infrastructure.

---

### Task 1: Match Qianfan request contracts

**Files:**
- Modify: `relay/channel/task/qianfan/adaptor.go`
- Test: `relay/channel/task/qianfan/adaptor_test.go`

- [ ] **Step 1: Add failing tests for official duration and parameter normalization**

Add deterministic tests asserting that native `duration: "3"` is accepted for K3.0/K3O billing projection, compatibility `resolution` maps to `mode` for K3.0/K3O, invalid modes/resolutions fail locally, Qianfan durations outside 3..15 fail locally, and Turbo accepts only `720p`/`1080p`.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run: `go test ./relay/channel/task/qianfan -run 'TestNormalize|TestValidateCompatibility' -count=1`

Expected: failures showing string duration rejection and missing compatibility validation.

- [ ] **Step 3: Implement Qianfan-only normalization**

Update `positiveInteger` to parse bounded decimal strings without changing the native body. In `ValidateRequestAndSetAction`, enforce duration 3..15, map K3.0/K3O `resolution` values (`720p`, `1080p`, `4k`) to (`std`, `pro`, `4k`), apply official defaults (`std` for K3.0 and `pro` for K3O), validate `mode`, and validate Turbo `resolution`.

- [ ] **Step 4: Emit the official Omni image structure**

For K3O compatibility image requests, emit:

```json
"image_list": [
  {
    "type": "first_frame",
    "image_url": "https://example.com/first.jpg"
  }
]
```

Do not change the K3.0 `image` field or Turbo `contents` structure.

- [ ] **Step 5: Run focused tests**

Run: `go test ./relay/channel/task/qianfan -count=1`

Expected: PASS.

### Task 2: Complete Qianfan task responses

**Files:**
- Modify: `relay/channel/task/qianfan/adaptor.go`
- Test: `relay/channel/task/qianfan/adaptor_test.go`

- [ ] **Step 1: Add a failing response contract test**

Assert that a completed task response contains the original public model name, top-level `url`, top-level `video_url`, matching `metadata.url`, and second-based timestamps.

- [ ] **Step 2: Run the response test and confirm failure**

Run: `go test ./relay/channel/task/qianfan -run TestConvertToOpenAIVideo -count=1`

Expected: failure because the current converter leaves `model`, `url`, and `video_url` empty and preserves millisecond timestamps.

- [ ] **Step 3: Implement the response contract**

Populate `Model` from `task.Properties.OriginModelName`, populate all three URL locations from the successful upstream result, and divide millisecond timestamps by 1000 when their value is in millisecond range.

- [ ] **Step 4: Run focused tests**

Run: `go test ./relay/channel/task/qianfan -count=1`

Expected: PASS.

### Task 3: Support Range requests for Baidu V2 video content

**Files:**
- Modify: `controller/video_proxy.go`
- Test: `controller/video_proxy_test.go`

- [ ] **Step 1: Add a failing Baidu V2 Range regression test**

Create an upstream server returning `206`, `Content-Range`, and `Accept-Ranges`; assert that a Baidu V2 task forwards `Range`/`If-Range`, preserves `206`, and streams only the partial body.

- [ ] **Step 2: Run the focused test and confirm failure**

Run: `go test ./controller -run TestVideoProxyBaiduV2ForwardsRange -count=1`

Expected: failure because the current default proxy neither forwards Range nor accepts 206.

- [ ] **Step 3: Implement a channel-scoped Range branch**

Introduce a local `forwardRange` condition for `constant.ChannelTypeBaiduV2`. Reuse the existing partial-content status and header forwarding behavior without enabling URL replacement or changing other channel types.

- [ ] **Step 4: Run the focused test**

Run: `go test ./controller -run 'TestVideoProxy(BaiduV2ForwardsRange|Privacy)' -count=1`

Expected: PASS, subject to the repository's pre-existing controller-package build state.

### Task 4: Expose the native Qianfan route

**Files:**
- Modify: `deploy/nginx/api.conf`

- [ ] **Step 1: Add an isolated API location**

Add `location ^~ /qianfan/` using the same worker upstream, buffering, timeouts, and forwarded headers as `/v1/`.

- [ ] **Step 2: Validate Nginx configuration syntax**

Run against the deployment container or a local Nginx image: `nginx -t`.

Expected: configuration syntax is valid.

### Task 5: Final verification

**Files:**
- Review: all modified files

- [ ] **Step 1: Format Go files**

Run: `gofmt -w relay/channel/task/qianfan/adaptor.go relay/channel/task/qianfan/adaptor_test.go controller/video_proxy.go controller/video_proxy_test.go`

- [ ] **Step 2: Run Qianfan tests**

Run: `go test ./relay/channel/task/qianfan -count=1`

Expected: PASS.

- [ ] **Step 3: Run broader build checks**

Run: `go test ./relay/channel/task/qianfan ./relay/common -count=1` and `go build ./...`.

Expected: PASS unless a documented pre-existing unrelated worktree failure remains.

- [ ] **Step 4: Review isolation**

Confirm the diff changes only Qianfan/Baidu V2 behavior and the explicit `/qianfan/` Nginx route, with no model registration, billing expression, database, or other provider changes.

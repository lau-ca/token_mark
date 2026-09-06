# Onfishes Seedance Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a registered Doubao Video channel support all sixteen documented onfishes Seedance video, liveness, asset, and asset-group interfaces while preserving existing VolcEngine and `/v1` behavior.

**Architecture:** Add native `/v3/contents/generations/tasks` handlers around the existing async task lifecycle and extend the Doubao adaptor with channel-specific upstream paths and native request/response conversion. Route `/v1/volc/ark` through a registered Doubao Video channel using the existing asset proxy and synthetic per-call billing model, while retaining the legacy environment-backed `/volc/ark` route.

**Tech Stack:** Go 1.22+, Gin, GORM v2, testify, existing relay/task and billing infrastructure.

---

### Task 1: Native Seedance request model and validation

**Files:**
- Modify: `relay/common/relay_info.go`
- Modify: `relay/common/relay_utils.go`
- Test: `relay/common/relay_utils_test.go`

- [ ] **Step 1: Write failing tests for documented native fields**

Add deterministic request tests proving that `content`, `callback_url`, `return_last_frame`, `execution_expires_after`, `generate_audio`, `draft`, `tools`, `safety_identifier`, `priority`, `resolution`, `ratio`, `duration`, `frames`, `seed`, `camera_fixed`, and `watermark` survive parsing, including explicit `false`, `0`, and `duration=-1`.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run: `go test ./relay/common -run 'Test.*Seedance.*Native' -count=1`

Expected: FAIL because the unified task DTO currently drops native `content` and optional pointer fields.

- [ ] **Step 3: Extend `TaskSubmitReq` without breaking existing task formats**

Add Seedance-native fields using pointer types for optional scalars. Keep the existing custom `UnmarshalJSON` duration handling, but preserve `duration=-1` for the native `/v3` route.

- [ ] **Step 4: Add route-aware validation**

For `/v3/contents/generations/tasks`, require `model` and a meaningful `text`, `image_url`, or `video_url` item; validate documented resolution, ratio, priority, execution-expiry, and duration ranges. Leave existing `/v1` request validation unchanged.

- [ ] **Step 5: Run tests**

Run: `go test ./relay/common -count=1`

Expected: PASS.

### Task 2: Doubao adaptor path, conversion, statuses, and native response

**Files:**
- Modify: `relay/channel/task/doubao/adaptor.go`
- Create: `relay/channel/task/doubao/adaptor_test.go`

- [ ] **Step 1: Write failing adaptor tests**

Cover:

- Doubao Video uses `/v3/contents/generations/tasks`;
- VolcEngine keeps `/api/v3/contents/generations/tasks`;
- fetch follows the same channel-specific rule;
- native `content`, roles, and `draft_task.id` are preserved;
- unified `prompt`, images, reference videos, and reference audios still convert correctly;
- `cancelled` and `expired` are terminal failures;
- native response retains all documented task fields and upstream ID.

- [ ] **Step 2: Run tests and confirm failure**

Run: `go test ./relay/channel/task/doubao -count=1`

Expected: FAIL on paths, native conversion, status handling, and missing response fields.

- [ ] **Step 3: Implement channel-specific task paths**

Use `TaskAdaptor.ChannelType` to select `/v3/...` only for `ChannelTypeDoubaoVideo`; preserve `/api/v3/...` for `ChannelTypeVolcEngine`. Do not inspect the hostname.

- [ ] **Step 4: Implement native conversion**

Extend `ContentItem` with `draft_task`, copy native content without reordering, and retain existing unified conversion as a fallback. Map top-level documented fields directly and use metadata only for compatibility fields not explicitly provided.

- [ ] **Step 5: Implement complete response parsing**

Retain `last_frame_url`, priority, draft, generate-audio, execution-expiry, usage, timestamps, and errors. Map `cancelled` and `expired` to terminal local failure while retaining the native status in stored response data.

- [ ] **Step 6: Run adaptor tests**

Run: `go test ./relay/channel/task/doubao -count=1`

Expected: PASS.

### Task 3: Native `/v3` task routes and handlers

**Files:**
- Modify: `router/video-router.go`
- Modify: `middleware/distributor.go`
- Modify: `controller/relay.go`
- Modify: `relay/relay_task.go`
- Modify: `model/task.go`
- Create: `controller/seedance_video.go`
- Create: `controller/seedance_video_test.go`

- [ ] **Step 1: Write failing route and handler tests**

Cover route registration, exact create response, direct task response without `{code,data}`, user-scoped lookup by upstream ID, list pagination and all documented filters, and delete forwarding.

- [ ] **Step 2: Run tests and confirm failure**

Run: `go test ./controller ./router -run 'Test.*Seedance|Test.*V3Contents' -count=1`

Expected: FAIL because the routes and handlers do not exist.

- [ ] **Step 3: Register native routes**

Add the four documented method/path combinations under a `/v3` group using existing Bearer token authentication. POST uses distribution; GET list is user-local; GET/DELETE by ID resolve the stored task and its channel.

- [ ] **Step 4: Preserve upstream IDs for `/v3` tasks**

After successful upstream creation and before insertion, use the upstream ID as `Task.TaskID` only for the native `/v3` route. Keep generated public IDs for existing `/v1` routes.

- [ ] **Step 5: Add cross-database list query**

Use GORM filters for user ID, Doubao platform, status, task IDs, model, service tier, limit, and offset. Parse provider-only response fields in Go rather than using database-specific JSON SQL.

- [ ] **Step 6: Add delete/cancel handler**

Verify ownership and Doubao channel type, build the upstream DELETE URL with the stored upstream ID, use the channel key and proxy, parse the returned native task, update local data/status, and return the native object.

- [ ] **Step 7: Run tests**

Run: `go test ./controller ./router ./model -run 'Test.*Seedance|Test.*V3Contents' -count=1`

Expected: PASS.

### Task 4: Channel-backed liveness and asset proxy

**Files:**
- Modify: `router/video-router.go`
- Modify: `middleware/distributor.go`
- Modify: `controller/seedance_asset_proxy.go`
- Modify: `controller/seedance_asset_proxy_test.go`

- [ ] **Step 1: Write failing tests**

Cover all twelve allowed Actions, rejection of unknown Actions, `/v1/volc/ark` route registration, registered-channel base URL/key/proxy use, `/v1/volc/ark` path append, query preservation, safe header forwarding, settlement on 2xx, and refund on failure.

- [ ] **Step 2: Run tests and confirm failure**

Run: `go test ./controller ./router -run 'TestSeedanceAsset|Test.*VolcArk' -count=1`

Expected: FAIL because the documented alias and channel-backed selection do not exist.

- [ ] **Step 3: Add asset distribution context**

Teach the distributor that `/v1/volc/ark` uses the internal model `seedance-asset-library`, allowing normal group/channel selection and token restrictions.

- [ ] **Step 4: Use the registered channel upstream**

For `/v1/volc/ark`, read selected base URL, key, and proxy from relay context, require `ChannelTypeDoubaoVideo`, and append the documented upstream path. Keep the existing environment-backed behavior only for `/volc/ark`.

- [ ] **Step 5: Run tests**

Run: `go test ./controller ./router ./middleware -run 'TestSeedanceAsset|Test.*VolcArk' -count=1`

Expected: PASS.

### Task 5: Regression verification and documentation re-audit

**Files:**
- Modify only if a verified mismatch is found: `docs/superpowers/specs/2026-08-17-onfishes-seedance-compatibility-design.md`

- [ ] **Step 1: Format changed Go files**

Run: `gofmt -w <only changed Go files>`

- [ ] **Step 2: Run focused package tests**

Run: `go test ./relay/common ./relay/channel/task/doubao ./controller ./router ./middleware ./model -count=1`

Expected: PASS, or unrelated pre-existing failures documented separately.

- [ ] **Step 3: Run root build**

Run: `go build ./...`

Expected: PASS.

- [ ] **Step 4: Revisit all sixteen documentation menus**

For every menu, compare method, path, Bearer authentication, request fields, response fields, status values, and passthrough behavior. Do not create a paid video task.

- [ ] **Step 5: Review the final diff**

Confirm only scoped files changed and no existing user modifications were overwritten. Do not stage or commit unless explicitly requested.

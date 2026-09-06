# Qianfan K3 Unified Video Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `Kling 3.0 Omni` work through the existing `/v1/videos` route and recognize the observed successful `Kling 3.0 Turbo` polling response without changing other channels.

**Architecture:** Keep the existing Baidu V2 task adaptor and public OpenAI-compatible video routes. Add model-specific request shaping only when the selected upstream model is `K3O`, and extend only the Qianfan result parser to accept the observed Turbo success shape (`message=SUCCEED`, empty `task_status`, non-empty video result).

**Tech Stack:** Go, Gin, testify, existing task relay and polling framework.

---

### Task 1: Lock the observed provider contracts with regression tests

**Files:**
- Modify: `relay/channel/task/qianfan/adaptor_test.go`

- [ ] **Step 1: Add a failing unified-route Omni request test**

Create a `TaskSubmitReq` for upstream model `K3O` and assert that `BuildRequestBody` emits `type: omni-video`, a string duration, the prompt, mode, aspect ratio, and `sound: off`.

- [ ] **Step 2: Add a failing Turbo polling success test**

Pass the observed response shape with `message: SUCCEED`, an empty `task_status`, a non-empty video result, and usage credits to `ParseTaskResult`; assert success status, URL, and final price.

- [ ] **Step 3: Run the focused tests and confirm they fail**

Run: `go test ./relay/channel/task/qianfan -run 'TestBuildRequestBodyUsesOmniShape|TestParseTaskResultAcceptsTurboSuccessMessage' -count=1`

Expected: the Omni request is emitted as `text2video`, and the Turbo response is rejected as an unknown empty status.

### Task 2: Implement the isolated Qianfan adaptor fix

**Files:**
- Modify: `relay/channel/task/qianfan/adaptor.go`

- [ ] **Step 1: Emit the official Omni request shape**

In `BuildRequestBody`, branch on upstream model `K3O`, set the type to `omni-video`, reuse the direct K3 fields, preserve the string duration required by Qianfan, and default sound to `off` for the compatibility route.

- [ ] **Step 2: Recognize the observed Turbo terminal response**

In `ParseTaskResult`, treat an empty `task_status` as success only when `message` equals `SUCCEED` case-insensitively and the first video URL is non-empty. Continue rejecting unknown empty responses.

- [ ] **Step 3: Run the focused tests**

Run: `go test ./relay/channel/task/qianfan -count=1`

Expected: PASS.

### Task 3: Verify shared task and billing behavior

**Files:**
- Verify: `relay/channel/task/qianfan/adaptor.go`
- Verify: `service/task_polling.go`
- Verify: `relay/relay_task.go`

- [ ] **Step 1: Format the changed Go files**

Run: `gofmt -w relay/channel/task/qianfan/adaptor.go relay/channel/task/qianfan/adaptor_test.go`

- [ ] **Step 2: Run affected backend tests**

Run: `go test ./relay/channel/task/qianfan ./relay ./service -count=1`

Expected: PASS.

- [ ] **Step 3: Review the exact diff**

Confirm the new behavior is confined to the Qianfan adaptor and its tests, and preserve all unrelated pre-existing worktree changes.

### Task 4: Deploy only production master

**Files:**
- Read only: `/data/frimodel/master/docker-compose.yml` on `148.113.178.75`

- [ ] **Step 1: Record master and worker container/image IDs and back up master**

Save the current master compose file and image under a timestamped `/data/frimodel/backups/` directory. Do not recreate the worker.

- [ ] **Step 2: Build and transfer a linux/amd64 image**

Create a clean temporary build context containing the intended source changes, build `frimodel/new-api:latest`, transfer the tar, verify SHA256, and load it on the server.

- [ ] **Step 3: Recreate only the master service**

Run: `sudo docker compose -f /data/frimodel/master/docker-compose.yml up -d --force-recreate new-api-master`

- [ ] **Step 4: Verify deployment isolation and health**

Confirm master is healthy, public status endpoints return 200, and the worker container ID and image ID are unchanged.

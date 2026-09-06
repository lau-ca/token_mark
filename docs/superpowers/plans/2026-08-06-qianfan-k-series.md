# Qianfan K-Series Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete all documented Qianfan K-series operations and expression billing through the existing Baidu V2 channel.

**Architecture:** Preserve native Qianfan bodies on the dedicated route, normalize only bounded billing metadata, and reuse the existing asynchronous task framework. Isolate synchronous face identification and Qianfan final-deduction settlement inside the Qianfan adaptor.

**Tech Stack:** Go 1.22+, Gin, existing task relay framework, billingexpr, testify.

---

### Task 1: Lock the Qianfan request contracts with tests

**Files:**
- Create: `relay/channel/task/qianfan/adaptor_test.go`
- Modify: `relay/channel/task/qianfan/adaptor.go`
- Modify: `relay/channel/task/qianfan/dto.go`

- [ ] Add table tests for supported models and operations, direct K3.0 fields, Turbo nested settings, `sound`, `video_list`, `voice_list`, duration bounds, and face-identification responses.
- [ ] Run `go test ./relay/channel/task/qianfan` and confirm the new contract tests fail before implementation.
- [ ] Add `K3O`, accept all documented native operation types, enforce model/type compatibility, and preserve native bodies.
- [ ] Split standard K3.0 request conversion from Turbo conversion.
- [ ] Parse synchronous face responses and terminal task state.
- [ ] Run `go test ./relay/channel/task/qianfan` and confirm it passes.

### Task 2: Correct Qianfan resource proxy contracts

**Files:**
- Create: `controller/qianfan_resource_test.go`
- Modify: `controller/qianfan_resource.go`

- [ ] Add request-builder tests for subject type selection, official singular voice identifiers, list paths, and pagination limits/defaults.
- [ ] Run the focused controller tests and confirm they fail.
- [ ] Correct identifiers and forward validated pagination to every documented list operation.
- [ ] Run the focused controller tests and confirm they pass.

### Task 3: Complete expression billing and final settlement

**Files:**
- Modify: `relay/helper/task_price.go`
- Modify: `relay/helper/task_price_test.go`
- Modify: `relay/channel/task/qianfan/adaptor.go`
- Modify: `relay/channel/task/qianfan/dto.go`
- Modify: `service/task_polling.go`
- Modify: `service/task_billing_test.go`
- Modify: `relay/relay_task.go`

- [ ] Add tests proving official string sound values and reference/voice lists reach expression parameters.
- [ ] Add tests proving Qianfan final deductions can settle an expression-billed task while unrelated expression/per-call tasks retain current behavior.
- [ ] Normalize the documented fields and parse final deductions with checked quota conversion.
- [ ] Permit Qianfan's adaptor-provided final quota to settle through the existing CAS-protected completion path.
- [ ] Initialize terminal state for synchronous face identification.
- [ ] Run the affected helper, service, and relay package tests.

### Task 4: Final verification

**Files:**
- Review all modified files only.

- [ ] Run `gofmt` on modified Go files.
- [ ] Run `go test ./relay/channel/task/qianfan ./relay/helper ./controller ./service`.
- [ ] Run `go test ./relay/... ./controller/... ./service/...` if focused tests pass.
- [ ] Inspect `git diff --check`, `git diff --stat`, and the exact diff, excluding all unrelated pre-existing changes.
- [ ] Confirm no non-Baidu channel branch or global route contract changed.

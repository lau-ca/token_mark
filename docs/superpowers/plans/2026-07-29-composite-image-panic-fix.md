# Composite Image Panic Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the composite image route nil-pointer panic, guarantee pre-consumed quota is refunded on unexpected composite-route panics, and verify retries and billing remain isolated from ordinary model calls.

**Architecture:** Keep the change inside `relayCompositeImage`. Reset each composite attempt with a valid empty `ChannelMeta` instead of a nil embedded pointer, and add a composite-only panic defer that refunds an active billing session before re-panicking to the existing HTTP recovery middleware. Tests exercise the reset and panic-refund contracts without changing common relay, channel distribution, pricing, or billing implementations.

**Tech Stack:** Go, Gin, GORM, testify, existing `BillingSession` and relay test fixtures.

---

### Task 1: Protect composite retry state reset

**Files:**
- Modify: `controller/composite_image_relay.go`
- Test: `controller/composite_image_relay_test.go`

- [ ] **Step 1: Write a failing retry-state regression test**

Add a deterministic test that creates a `relaycommon.RelayInfo` with populated channel mapping and conversion state, invokes the composite-attempt reset, and asserts that `ChannelMeta` remains non-nil while channel mapping and conversion fields are cleared.

- [ ] **Step 2: Run the targeted test and verify it fails**

Run: `go test ./controller -run TestResetCompositeRelayAttempt -count=1`

Expected: FAIL because the reset function does not exist or reproduces the nil embedded-pointer panic.

- [ ] **Step 3: Implement the isolated reset**

Create a composite-specific reset function that assigns `&relaycommon.ChannelMeta{}` and clears `RequestConversionChain`, `FinalRequestRelayFormat`, `RetryIndex`, and `LastError`. Call it at the start of every configured retry attempt. Do not modify `RelayInfo.InitChannelMeta`, the distributor, or ordinary relay handlers.

- [ ] **Step 4: Run the targeted test and verify it passes**

Run: `go test ./controller -run TestResetCompositeRelayAttempt -count=1`

Expected: PASS.

### Task 2: Refund pre-consumed quota on an unexpected composite panic

**Files:**
- Modify: `controller/composite_image_relay.go`
- Test: `controller/composite_image_relay_test.go`

- [ ] **Step 1: Write a failing panic-refund regression test**

Add a composite-only panic guard test using a billing-settler spy. Trigger a panic after the billing object is attached, assert `Refund` is called exactly once, and assert the original panic is rethrown for the existing recovery middleware.

- [ ] **Step 2: Run the targeted test and verify it fails**

Run: `go test ./controller -run TestCompositePanicRefund -count=1`

Expected: FAIL because the current composite defer only refunds when `newAPIError` is non-nil.

- [ ] **Step 3: Implement the composite-only panic guard**

Add a defer after `relayInfo` creation. On panic, call `relayInfo.Billing.Refund(c)` when a billing session exists, then re-panic with the original value. Leave global recovery and all ordinary relay paths unchanged.

- [ ] **Step 4: Run the targeted test and verify it passes**

Run: `go test ./controller -run 'TestResetCompositeRelayAttempt|TestCompositePanicRefund' -count=1`

Expected: PASS.

### Task 3: Validate retry and billing isolation

**Files:**
- Test: `controller/composite_image_relay_test.go`
- Test: `service/composite_group_test.go`
- Test: `service/composite_billing_test.go`

- [ ] **Step 1: Run composite routing and billing tests**

Run: `go test ./controller ./service -run 'Composite|BillingSession' -count=1`

Expected: PASS.

- [ ] **Step 2: Run race detection on the changed packages**

Run: `go test -race ./controller ./service -run 'Composite|BillingSession' -count=1`

Expected: PASS with no race report.

- [ ] **Step 3: Run broader relay and pricing regression tests**

Run: `go test ./controller ./middleware ./relay/... ./service -count=1`

Expected: PASS.

### Task 4: Verify against production configuration from a local worker

**Files:**
- No source changes.

- [ ] **Step 1: Start an isolated local worker**

Connect to the production PostgreSQL configuration through an SSH tunnel, use a unique local node name, disable batch updates and channel tests, and listen on a local-only port.

- [ ] **Step 2: Run a single composite generation request**

Send the approved `gpt-image-2` request to the local worker and verify there is no panic, the selected physical route is recorded, and billing settles or refunds exactly once.

- [ ] **Step 3: Run staged composite-only concurrency**

Increase concurrency in bounded stages while keeping the total request count at or below 1000. Stop immediately on any panic, duplicate settlement, negative/incorrect quota delta, ordinary-group routing, or unexpected physical channel selection.

- [ ] **Step 4: Verify ordinary channels remain untouched**

Compare ordinary relay source diffs, channel error/consume logs, and channel selection metadata before and after the test. Confirm the code diff only changes the composite controller and its tests, and that load requests used only the configured composite public group/model.

- [ ] **Step 5: Stop the local worker and remove its system-instance record**

Gracefully stop the local process, close the SSH tunnel, and delete only the exact temporary node record created by the local debug instance.

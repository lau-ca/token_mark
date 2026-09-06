# Channel Retry All Failures Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a positive per-channel `retry_times` retry every unsuccessful synchronous relay attempt on the same channel without filtering by HTTP status code or skip-retry marker.

**Architecture:** Keep the existing nested same-channel retry loop and change only its eligibility predicate. The global cross-channel retry loop remains unchanged and continues to use the configured automatic retry status codes. Preserve the two non-replayable boundaries: no retry after a response is written and no retry after the client request context is canceled.

**Tech Stack:** Go 1.22+, Gin, testify, Docker Buildx, Docker Compose

---

### Task 1: Protect the new retry contract with unit tests

**Files:**
- Modify: `controller/relay_channel_retry_test.go`

- [ ] **Step 1: Add cases for HTTP 400, skip-retry markers, and canceled clients**

Extend `TestShouldRetrySameChannel` so an HTTP 400 error and an error carrying `ErrOptionWithSkipRetry()` both expect `true` while retries remain. Add a `canceled` fixture field, create a canceled request context for that case, and expect `false`.

- [ ] **Step 2: Run the focused test before implementation**

Run: `go test ./controller -run 'Test(GetChannelRetryTimes|ShouldRetrySameChannel)$' -count=1`

Expected: the new HTTP 400 and skip-retry cases fail because the current predicate still filters those errors.

### Task 2: Make same-channel retries independent of error classification

**Files:**
- Modify: `controller/relay.go:315`

- [ ] **Step 1: Replace the classification checks with replayability checks**

Keep the existing guards for exhausted retries, missing errors, and a written response. Add a guard for `c.Request.Context().Err() != nil` when a request exists. Return `true` for every remaining error after those guards.

- [ ] **Step 2: Format and run focused tests**

Run: `gofmt -w controller/relay.go controller/relay_channel_retry_test.go`

Run: `go test ./controller -run 'Test(GetChannelRetryTimes|ShouldRetrySameChannel)$' -count=1`

Expected: PASS.

- [ ] **Step 3: Run affected backend verification**

Run: `go test ./controller -count=1`

Run: `cd relaykit && GOWORK=off go build ./...`

Expected: both commands pass.

### Task 3: Build a clean production image

**Files:**
- Read: `Dockerfile`
- Create outside the repository: temporary clean build context and image archive

- [ ] **Step 1: Inspect the destination before changing it**

Connect with `ssh ubuntu@148.113.178.75` and inspect running containers, the master compose file/path, configured image name, current image ID, health, and architecture. Do not restart any service during inspection.

- [ ] **Step 2: Create a clean build context containing the tracked tree plus intended working changes**

Use a temporary directory and copy the repository without `.git`, build artifacts, test results, deployment archives, or unrelated untracked files. Verify that the resulting `controller/relay.go` contains the new predicate.

- [ ] **Step 3: Build and export the target architecture image**

Build with Docker Buildx for the destination architecture, tag it with the exact image reference used by the master compose service, export it as a compressed tar archive, and calculate SHA-256.

### Task 4: Deploy only the master service and verify production

**Files:**
- Remote compose file discovered in Task 3

- [ ] **Step 1: Prepare rollback before switching**

Record the current master container image ID and tag the current image with a timestamped rollback tag. Validate the compose file with `docker compose config --quiet`.

- [ ] **Step 2: Upload and load the new image**

Upload the archive to the destination host, verify SHA-256, and load it with Docker. Confirm the loaded image ID differs from the rollback image ID.

- [ ] **Step 3: Recreate only `new-api-master`**

Run Docker Compose with the discovered compose file and `up -d --force-recreate --no-deps new-api-master`. Do not recreate or restart worker, CPA, nginx, database, or Redis services.

- [ ] **Step 4: Verify service health and isolation**

Wait for `new-api-master` to become running/healthy, check recent master logs, call the local/public `/api/status` endpoint, and confirm sibling container IDs/start times are unchanged.

- [ ] **Step 5: Report the deployed image, rollback tag, and verification results**

Include the exact running image ID, artifact checksum, health endpoint result, and confirmation that only master changed.

# User Quota Exhaustion Forecast Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a seven-day weighted quota-exhaustion forecast to visible admin user rows and the ordinary-user dashboard without slowing the existing user-list request.

**Architecture:** A new backend forecast service reads hourly `quota_data` aggregates for only requested users, calculates rolling weighted usage in Go, and caches results for five minutes while validating the current quota snapshot. Separate admin batch and authenticated self endpoints keep forecast work outside the existing user-list APIs. The frontend loads forecasts independently with React Query and shares formatting logic between the user table and dashboard.

**Tech Stack:** Go 1.22, Gin, GORM v2, testify, React 19, TypeScript, TanStack React Query, i18next, Bun, Vitest.

---

### Task 1: Add forecast persistence queries and index

**Files:**
- Create: `model/quota_forecast.go`
- Create: `model/quota_forecast_test.go`
- Modify: `model/usedata.go`

- [ ] **Step 1: Write model tests for multi-user inputs and hourly aggregation**

Create SQLite-backed tests that insert users plus multiple `quota_data` rows and assert that one query path returns current quota, creation time, and `SUM(quota)` grouped by `user_id, created_at`. Include a requested user with no usage rows.

- [ ] **Step 2: Run the model test and verify it fails**

Run: `go test ./model -run 'TestGetQuotaForecast' -count=1`

Expected: FAIL because the forecast query types and functions do not exist.

- [ ] **Step 3: Implement dialect-neutral query functions**

Create domain query types:

```go
type QuotaForecastUser struct {
    UserID    int
    Quota     int
    CreatedAt int64
}

type QuotaForecastUsage struct {
    UserID    int
    CreatedAt int64
    Quota     int64
}
```

Implement a user query over `DB` and an hourly grouped usage query over `quota_data`, using GORM `Where("user_id IN ?", ids)`, timestamp bounds, `SUM(quota)`, and `Group("user_id, created_at")`. Add `idx_qdt_user_created` GORM tags to `QuotaData.UserID` and `QuotaData.CreatedAt` while retaining existing indexes.

- [ ] **Step 4: Run model tests**

Run: `go test ./model -run 'TestGetQuotaForecast' -count=1`

Expected: PASS.

### Task 2: Implement weighted forecast calculation and bounded cache

**Files:**
- Create: `service/quota_forecast.go`
- Create: `service/quota_forecast_test.go`

- [ ] **Step 1: Write calculation tests**

Cover full seven-window weights `7..1`, account-age normalization, partial oldest-window prorating, depleted quota, less than 24 hours of history, seven days with zero consumption, ignoring usage older than seven days, timestamp overflow, cache hits, and cache invalidation when quota changes.

- [ ] **Step 2: Run service tests and verify they fail**

Run: `go test ./service -run 'Test.*QuotaForecast' -count=1`

Expected: FAIL because forecast calculation and service functions do not exist.

- [ ] **Step 3: Implement the forecast service**

Define statuses `predicted`, `depleted`, `sampling`, `no_recent_usage`, and `unavailable`. Use seven rolling 24-hour windows ending at a supplied calculation timestamp. Normalize the denominator for account age, return `no_recent_usage` when the seven-day total is zero, and never inspect older rows. Guard all float and timestamp conversions.

Implement a process-local mutex-protected cache keyed by user ID with a five-minute TTL and a bounded maximum entry count. Cache entries include the quota snapshot; only entries whose quota still matches may be reused. Query usage only for cache misses.

- [ ] **Step 4: Run service tests**

Run: `go test ./service -run 'Test.*QuotaForecast' -count=1`

Expected: PASS.

### Task 3: Add admin batch and self endpoints

**Files:**
- Create: `controller/quota_forecast.go`
- Create: `controller/quota_forecast_test.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write controller tests**

Test malformed JSON, empty IDs, non-positive IDs, deduplication, the 100-unique-ID limit, admin authorization routing, authenticated self scoping, and successful API response shapes.

- [ ] **Step 2: Run controller tests and verify they fail**

Run: `go test ./controller -run 'Test.*QuotaForecast' -count=1`

Expected: FAIL because handlers and routes do not exist.

- [ ] **Step 3: Implement handlers and routes**

Add `POST /api/user/quota-forecast` under `AdminAuth` and `GET /api/user/self/quota-forecast` under `UserAuth`. Decode JSON through `common.DecodeJson`, validate and deduplicate IDs, call the shared service with `common.GetTimestamp()`, and use the project's standard success/error response wrappers.

- [ ] **Step 4: Run controller and router tests**

Run: `go test ./controller ./router -run 'Test.*QuotaForecast' -count=1`

Expected: PASS.

### Task 4: Add shared frontend forecast API and presentation helpers

**Files:**
- Create: `web/default/src/features/quota-forecast/types.ts`
- Create: `web/default/src/features/quota-forecast/api.ts`
- Create: `web/default/src/features/quota-forecast/lib.ts`
- Create: `web/default/src/features/quota-forecast/lib.test.ts`
- Create: `web/default/src/features/quota-forecast/quota-forecast-display.tsx`

- [ ] **Step 1: Write formatting tests**

Test status labels, remaining-duration formatting, one-year display cap, and destructive/warning/muted threshold selection.

- [ ] **Step 2: Run the frontend unit test and verify it fails**

Run from `web/default`: `bun test src/features/quota-forecast/lib.test.ts`

Expected: FAIL because the shared forecast module does not exist.

- [ ] **Step 3: Implement API, types, formatting, and display component**

Add typed admin batch and self requests. Implement pure presentation helpers plus a compact display component that supports inline skeleton, special states, localized exact timestamps, remaining duration, threshold colors, and tooltip details.

- [ ] **Step 4: Run the frontend unit test**

Run: `bun test src/features/quota-forecast/lib.test.ts`

Expected: PASS.

### Task 5: Load forecasts asynchronously on `/users`

**Files:**
- Modify: `web/default/src/features/users/components/users-table.tsx`
- Modify: `web/default/src/features/users/components/users-columns.tsx`

- [ ] **Step 1: Add one current-page React Query**

After the normal user query returns, build a stable sorted ID list and request forecasts with a separate query key. Enable it only when IDs exist, use a five-minute `staleTime`, and suppress global error toasts.

- [ ] **Step 2: Pass forecasts to the column factory**

Change `useUsersColumns` to accept the forecast map plus loading/failure state. Map strictly by `row.original.id`.

- [ ] **Step 3: Render the shared forecast display in the quota cell**

Keep the current balance and progress bar unchanged. Add the compact forecast line underneath and increase only the quota cell's vertical spacing, not its width.

- [ ] **Step 4: Run targeted TypeScript and lint checks**

Run from `web/default`:

```bash
bun run typecheck
bunx oxlint src/features/quota-forecast src/features/users/components/users-table.tsx src/features/users/components/users-columns.tsx
```

Expected: no errors.

### Task 6: Replace the dashboard's local runway estimate

**Files:**
- Modify: `web/default/src/features/dashboard/components/overview/summary-cards.tsx`

- [ ] **Step 1: Add the self-forecast query**

Fetch `/api/user/self/quota-forecast` independently with a five-minute `staleTime`. Leave the existing 24-hour usage query and sparklines unchanged.

- [ ] **Step 2: Replace only the runway block**

Use the backend forecast for health state, remaining duration, and exact exhaustion time. Forecast failure must affect only this block and show the unavailable state.

- [ ] **Step 3: Run targeted checks**

Run from `web/default`:

```bash
bun run typecheck
bunx oxlint src/features/dashboard/components/overview/summary-cards.tsx
```

Expected: no errors.

### Task 7: Synchronize translations and validate the complete change

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify: `web/default/src/i18n/locales/zh-TW.json`
- Modify: `web/default/src/i18n/locales/_reports/_sync-report.json`

- [ ] **Step 1: Run i18n synchronization**

Run from `web/default`: `bun run i18n:sync`

Expected: every new English source key exists in all supported locale files.

- [ ] **Step 2: Run backend verification**

Run from repository root:

```bash
go test ./model ./service ./controller ./router -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend verification**

Run from `web/default`:

```bash
bun test src/features/quota-forecast/lib.test.ts
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 4: Check the patch**

Run: `git diff --check`

Expected: no whitespace errors.

### Task 8: Package and deploy only `new-api-master`

**Files:**
- Read: `/root/gateway/master/docker-compose.yml` on `157.230.213.186`

- [ ] **Step 1: Confirm live target and prepare recovery**

Read the current master compose definition, running image ID, container health, and `/api/status`. Record an explicit command that recreates `new-api-master` from the current image before switching anything.

- [ ] **Step 2: Build a clean Linux AMD64 image**

Create a temporary clean source archive from the committed feature state, build the repository Dockerfile for `linux/amd64`, and export the image as a tar. Do not include unrelated local uncommitted files.

- [ ] **Step 3: Transfer and load the image manually**

Copy the tar to `157.230.213.186`, load it with Docker, and verify the loaded image ID before changing the service.

- [ ] **Step 4: Recreate only the master service**

Run:

```bash
docker compose -f /root/gateway/master/docker-compose.yml up -d --force-recreate new-api-master
```

Leave `new-api-worker-local`, `cpa`, and `gateway-nginx` untouched.

- [ ] **Step 5: Verify deployment**

Wait for `new-api-master` to become healthy, confirm the running image ID, and verify both local and public `/api/status` plus authenticated forecast endpoint routing. If health fails, execute the prepared recovery command immediately.

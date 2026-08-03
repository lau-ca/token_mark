# User Key Usage Analytics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an ordinary-user-only dashboard tab that reports usage for every API key with the same filters, metrics, charts, and visual language as model call analytics.

**Architecture:** Add a user-scoped aggregate endpoint backed by `quota_data` and token metadata, then render the returned bucket rows through focused dashboard components. Keep authorization and aggregation on the server, while chart/table presentation and selected-range calculations remain in the React feature layer.

**Tech Stack:** Go, Gin, GORM v2, React 19, TypeScript, TanStack Query/Table, Base UI/shadcn components, Tailwind CSS, VChart, i18next, Bun.

---

### Task 1: Define and test user-scoped Key usage aggregation

**Files:**
- Create: `model/usedata_key.go`
- Create: `model/usedata_key_test.go`

- [ ] Add deterministic tests that seed two users, active/disabled/deleted tokens, and hourly quota rows, then assert current-user isolation, per-hour aggregation, zero-usage tokens, deleted-token fallback, and masked keys.
- [ ] Run `go test ./model -run 'TestGetUserKeyQuotaData' -count=1` and confirm the new tests fail before implementation.
- [ ] Implement `KeyQuotaData` and `GetUserKeyQuotaData(userID, startTime, endTime)` using GORM group queries and a second user-scoped token query.
- [ ] Run the targeted model tests and confirm they pass.

### Task 2: Add the authenticated API contract

**Files:**
- Modify: `controller/usedata.go`
- Modify: `router/api-router.go`
- Create: `controller/usedata_key_test.go`

- [ ] Add controller tests for valid data, invalid timestamps, reversed ranges, and ranges longer than 30 days.
- [ ] Add `GET /api/data/keys/self` behind `middleware.UserAuth()`.
- [ ] Implement `GetUserKeyQuotaDates` using the existing strict timestamp parser and ordinary-user one-month limit.
- [ ] Run `go test ./controller ./model -run 'KeyQuota|KeyQuotaDates' -count=1` and confirm it passes.

### Task 3: Register the ordinary-user-only dashboard section

**Files:**
- Modify: `web/default/src/features/dashboard/section-registry.tsx`
- Modify: `web/default/src/features/dashboard/index.tsx`
- Modify: `web/default/src/features/dashboard/api.ts`
- Modify: `web/default/src/features/dashboard/types.ts`

- [ ] Add the `keys` section and `Key Usage Analytics` metadata.
- [ ] Filter the section so it is visible only when `role === ROLE.USER`, and redirect non-user direct access to `models`.
- [ ] Reuse `modelFilters` and `ModelsFilter` for the section actions.
- [ ] Add the typed API query for `/api/data/keys/self`.

### Task 4: Build Key analytics presentation components

**Files:**
- Create: `web/default/src/features/dashboard/components/keys/key-usage-analytics.tsx`
- Create: `web/default/src/features/dashboard/components/keys/key-usage-charts.tsx`
- Create: `web/default/src/features/dashboard/components/keys/key-usage-table.tsx`
- Create: `web/default/src/features/dashboard/lib/key-usage.ts`
- Create: `web/default/src/features/dashboard/lib/key-usage.test.ts`

- [ ] Add pure aggregation tests for totals, Key identity, percentage, sorting, time buckets, zero rows, and deleted labels.
- [ ] Implement pure transformation functions used by both charts and table.
- [ ] Build the page shell with the existing bordered panels, stat-card proportions, Skeleton behavior, chart tabs, table styling, and responsive overflow patterns.
- [ ] Add Key-name search and sortable consumption/request/token columns without requesting full Key values.
- [ ] Lazy-load the analytics section from the dashboard parent.

### Task 5: Complete translations and frontend validation

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/zh-TW.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] Add translations for the tab, chart/table labels, statuses, deleted/zero-usage states, and search placeholder.
- [ ] Run `cd web/default && bun run i18n:sync` and review only feature-related locale changes.
- [ ] Run the targeted Vitest file, `bun run typecheck`, targeted `bunx oxlint`, and `bun run build`.
- [ ] Run `git diff --check` and the targeted Go test packages.

### Task 6: Isolated master-only deployment

**Files:**
- No repository source changes.

- [ ] Commit only this feature's source, tests, translations, spec, and plan; leave unrelated workspace changes unstaged.
- [ ] Export committed `HEAD` into a temporary clean build context and build a `linux/amd64` Docker image archive.
- [ ] Inspect `148.113.178.75` read-only to confirm the current `new-api-master` compose path, image reference, and health state.
- [ ] Transfer and load the new image, retaining the previous image ID for rollback.
- [ ] Recreate only `new-api-master` with `/root/gateway/master/docker-compose.yml`; do not touch worker, CPA, or Nginx.
- [ ] Verify the running image ID, container health, and `https://api.frimodel.com/api/status` before completion.

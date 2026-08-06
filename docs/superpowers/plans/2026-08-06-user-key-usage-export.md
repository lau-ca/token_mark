# User Key Usage Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a professional ordinary-user Excel export for Key usage, including overview, Key summaries, and Key-by-model details for the active dashboard time range.

**Architecture:** Aggregate user-scoped consume logs in Go from `model.LOG_DB`, merge current token metadata from the main database, and expose a typed JSON endpoint. The React dashboard requests that payload on demand and dynamically imports ExcelJS to build a localized, styled workbook without increasing the initial bundle.

**Tech Stack:** Go, Gin, GORM v2, React 19, TypeScript, TanStack Query, ExcelJS, i18next, Bun, Vitest.

---

### Task 1: Add user-scoped Key export aggregation

**Files:**
- Create: `model/usedata_key_export.go`
- Create: `model/usedata_key_export_test.go`

- [ ] Write deterministic tests using separate main and log databases for user isolation, Key × model aggregation, prompt/completion totals, zero-use current Keys, deleted Key history, and last-used timestamps.
- [ ] Run `go test ./model -run 'TestGetUserKeyUsageExport' -count=1` and confirm the tests fail before implementation.
- [ ] Implement `GetUserKeyUsageExport(userID, startTime, endTime)` using a GORM aggregate over `LOG_DB` with `type = LogTypeConsume`, then merge owned token metadata from `DB`.
- [ ] Sort Key rows by quota descending and model rows by Key identity then quota descending.
- [ ] Run the targeted model test and confirm it passes.

### Task 2: Expose the authenticated export data endpoint

**Files:**
- Modify: `controller/usedata.go`
- Modify: `router/api-router.go`
- Create: `controller/usedata_key_export_test.go`

- [ ] Add controller tests for the authenticated user boundary, invalid timestamps, reversed ranges, and ranges over 30 days.
- [ ] Add `GET /api/data/keys/self/export` behind `middleware.UserAuth()`.
- [ ] Reuse `parseQuotaTimeRange`, return the generated timestamp and the aggregate payload, and keep the existing one-month limit.
- [ ] Run `go test ./controller ./model -run 'KeyUsageExport|UserKeyUsageExport' -count=1`.

### Task 3: Build pure workbook data and filename helpers

**Files:**
- Create: `web/src/features/dashboard/lib/key-usage-export.ts`
- Create: `web/src/features/dashboard/lib/__tests__/key-usage-export.test.ts`
- Modify: `web/src/features/dashboard/types.ts`
- Modify: `web/src/features/dashboard/api.ts`

- [ ] Add typed export response, Key summary, and Key-model detail interfaces.
- [ ] Write failing tests for localized deleted-Key fallback, quota-to-display conversion, totals, percentage calculations, stable sorting, invalid filename replacement, and empty data.
- [ ] Implement the pure report model and filename helpers.
- [ ] Run `cd web && bun run test -- src/features/dashboard/lib/__tests__/key-usage-export.test.ts`.

### Task 4: Generate the styled Excel workbook on demand

**Files:**
- Create: `web/src/features/dashboard/lib/key-usage-workbook.ts`
- Modify: `web/package.json`
- Modify: `web/bun.lock`

- [ ] Add ExcelJS with Bun.
- [ ] Implement a workbook builder with `数据概览`, `Key 汇总`, and `模型明细` sheets, shared title/table styles, freezes, filters, widths, number formats, status colors, and zero-display formats.
- [ ] Keep the ExcelJS import dynamic from the click path so the initial dashboard bundle does not include it.
- [ ] Generate an `ArrayBuffer` and download it through a temporary object URL that is always revoked.

### Task 5: Add the dashboard export action and translations

**Files:**
- Modify: `web/src/features/dashboard/components/keys/key-usage-analytics.tsx`
- Modify: `web/src/features/dashboard/index.tsx`
- Create: `web/scripts/add-missing-keys.mjs` temporarily
- Modify through script: `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

- [ ] Add an export callback from `KeyUsageAnalytics` to the dashboard section action area so “导出报告” appears beside “筛选”.
- [ ] Show loading state, prevent duplicate clicks, and display localized success/error toasts.
- [ ] Populate every new UI key for all seven locales through `scripts/add-missing-keys.mjs`, run it, run `bun run i18n:sync`, and delete the temporary script.
- [ ] Add a focused component regression test if the action-state behavior cannot be protected by the pure helper test.

### Task 6: Validate and deploy only master

**Files:**
- No additional source files.

- [ ] Run targeted Go tests, targeted Vitest, `bun run typecheck`, targeted `bunx oxlint`, `bun run build`, and `git diff --check`.
- [ ] Stage and commit only this feature's source, tests, translations, design, plan, and dependency lock changes.
- [ ] Inspect `ubuntu@148.113.178.75` read-only to confirm the master compose path, current image and health state.
- [ ] Build a clean `linux/amd64` image from committed source, transfer/load it, and retain the previous image ID as the immediate recovery reference.
- [ ] Recreate only the master service; do not touch worker, CPA, Nginx, databases, or unrelated services.
- [ ] Verify the new image ID, container health, and the live `/api/status` endpoint.

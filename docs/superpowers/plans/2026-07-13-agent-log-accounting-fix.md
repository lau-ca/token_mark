# Agent Log Accounting Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make agent earnings and settlements use exact consume-log timestamps instead of hourly cached `quota_data`, and align summary cards with their displayed time scope.

**Architecture:** Query immutable consume logs from `model.LOG_DB`, grouped by user, group, and exact creation second, then reuse the existing assignment/configuration interval matcher. Keep settlements in the main database and require a short closed-period safety window so synchronous consume logs are present before confirmation. Use filtered statistics for period cards and cumulative statistics only for earnings, settled, and pending cards.

**Tech Stack:** Go, GORM v2, PostgreSQL/MySQL/SQLite/ClickHouse-compatible log queries, testify, React 19, TanStack Query, TypeScript, Bun.

---

### Task 1: Replace hourly quota-data input with consume logs

**Files:**
- Modify: `service/agent_stats.go`
- Modify: `service/agent_stats_test.go`

- [ ] Seed exact-timestamp `model.Log` consume rows in the log database and add a regression test where configuration and inviter ownership change inside one hour.
- [ ] Query `model.LOG_DB.Model(&model.Log{})` for `type = model.LogTypeConsume`, matching customer IDs and exact timestamp bounds, grouped by `user_id`, `username`, `group`, and `created_at`.
- [ ] Preserve the existing per-version group formula and pagination behavior.
- [ ] Run `go test ./service -run 'AgentStats' -count=1` and expect PASS.

### Task 2: Prevent settlement from racing recent log writes

**Files:**
- Modify: `service/agent_settlement.go`
- Modify: `service/agent_settlement_test.go`
- Modify: `web/default/src/features/agents/index.tsx`

- [ ] Add a shared settlement safety delay and reject cutoffs newer than the closed period.
- [ ] Set the frontend default cutoff to the latest valid closed time and keep administrator-selected cutoffs validated by the backend.
- [ ] Add tests for recent-cutoff rejection and exact post-cutoff inclusion in the next settlement.
- [ ] Run `go test ./service -run 'AgentSettlement' -count=1` and expect PASS.

### Task 3: Correct summary display scope

**Files:**
- Modify: `web/default/src/features/agents/index.tsx`

- [ ] Use filtered `stats.summary` for customer count and selected-period consumption.
- [ ] Keep cumulative `summary` for cumulative agent earnings, settled amount, pending amount, and administrator platform-retained amount.
- [ ] Ensure summary queries refresh as time advances rather than freezing `Date.now()` for the component lifetime.

### Task 4: Verify and commit

**Files:**
- Test: `service/agent_stats_test.go`
- Test: `service/agent_settlement_test.go`

- [ ] Run `go test ./model ./service ./controller -count=1`.
- [ ] Run `cd web/default && bun run typecheck` and targeted oxlint.
- [ ] Run `cd web/default && bun run build`.
- [ ] Run `git diff --check`, exclude unrelated video/deploy/test-results changes, and commit only the accounting fix.

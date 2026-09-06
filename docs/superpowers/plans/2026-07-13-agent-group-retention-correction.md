# Agent Group Retention Correction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the agent-wide platform-retention percentage with independent per-group retention percentages and limit configuration to globally user-selectable groups.

**Architecture:** Keep immutable effective configuration versions and real-time `quota_data` aggregation. Move platform retention onto each `AgentGroupMargin`, retain legacy version/profile columns only for migration compatibility, and backfill existing group rows from their version-level value. Add an admin endpoint that returns the intersection of `UserUsableGroups` and configured billing groups, excluding `auto`, then render two percentage inputs per group in `web/default`.

**Tech Stack:** Go, Gin, GORM v2, SQLite/MySQL/PostgreSQL, testify, React 19, TypeScript, TanStack Query, shadcn/Base UI, i18next, Bun.

---

### Task 1: Move retention to group rules

**Files:**
- Modify: `model/agent.go`
- Modify: `model/main.go`
- Modify: `model/agent_test.go`

- [ ] **Step 1: Add failing model tests**

Add deterministic tests proving that two groups in the same version can store different `PlatformRetentionRate` values and invalid per-group retention values are rejected.

- [ ] **Step 2: Run the focused tests**

Run: `go test ./model -run '^TestAgent' -count=1`

Expected: FAIL until `AgentGroupMargin` and `AgentGroupMarginInput` accept per-group retention.

- [ ] **Step 3: Implement the model change**

Add `PlatformRetentionRate float64` to `AgentGroupMargin` and `AgentGroupMarginInput`. Validate both group rates in `normalizeAgentGroupMargins` and persist both fields when creating a version. Stop using a request-supplied agent-wide retention rate for new versions while retaining the legacy columns for compatibility.

- [ ] **Step 4: Backfill existing rows cross-database safely**

After migration adds the new column, update only group rows whose per-group retention has not been migrated, copying `agent_margin_versions.platform_retention_rate`. Use GORM reads and batched updates rather than dialect-specific SQL. Ensure a legitimate configured zero retention is distinguishable after migration by adding a migration marker or by running a one-time idempotent migration keyed through the existing option system.

- [ ] **Step 5: Run model tests**

Run: `go test ./model -run '^TestAgent' -count=1`

Expected: PASS.

### Task 2: Use per-group retention in statistics and settlement

**Files:**
- Modify: `service/agent_stats.go`
- Modify: `service/agent_stats_test.go`
- Modify: `service/agent_settlement_test.go`

- [ ] **Step 1: Add failing calculation tests**

Create one version with groups such as `{codex: gross 30%, retention 9%}` and `{gpt_image_web: gross 12%, retention 19%}`. Assert exact gross profit, platform retention, and agent earnings for both groups and settlement totals.

- [ ] **Step 2: Run focused tests**

Run: `go test ./service -run 'AgentStats|AgentSettlement' -count=1`

Expected: FAIL while calculation still reads the version-level retention.

- [ ] **Step 3: Implement calculation change**

Build a map of complete group rules rather than only gross-margin values. Calculate platform retention with the matched group's `PlatformRetentionRate`; unconfigured groups continue to produce zero earnings.

- [ ] **Step 4: Run focused tests**

Run: `go test ./service -run 'AgentStats|AgentSettlement' -count=1`

Expected: PASS.

### Task 3: Expose only globally user-selectable groups

**Files:**
- Modify: `controller/agent.go`
- Modify: `router/api-router.go`
- Modify: `dto/agent.go`
- Modify: `controller/agent_test.go` or `model/agent_test.go`

- [ ] **Step 1: Add failing contract tests**

Assert that agent configuration accepts `{group, gross_margin_rate, platform_retention_rate}`, rejects groups absent from global `UserUsableGroups`, and that the admin group-list endpoint excludes `auto` and groups absent from `GroupRatio`.

- [ ] **Step 2: Add the group-list endpoint**

Add an administrator-only endpoint under `/api/agent/admin/groups`. Build a stable sorted list from `setting.GetUserUsableGroupsCopy()`, excluding `auto`, and retain only names present in `ratio_setting.GetGroupRatioCopy()`.

- [ ] **Step 3: Validate saved groups on the backend**

Before saving an enabled configuration, reject every group not present in the same globally user-selectable set. This prevents direct API calls from configuring internal groups.

- [ ] **Step 4: Update DTOs and run tests**

Remove the required request-level retention field and add `platform_retention_rate` to each group item.

Run: `go test ./controller ./model ./service -run 'Agent' -count=1`

Expected: PASS.

### Task 4: Update the default-frontend settings drawer

**Files:**
- Modify: `web/default/src/features/agents/types.ts`
- Modify: `web/default/src/features/agents/api.ts`
- Modify: `web/default/src/features/agents/components/agent-settings-drawer.tsx`
- Modify: `web/default/src/features/users/index.tsx`

- [ ] **Step 1: Update schemas and API types**

Make each group rule contain `gross_margin_rate` and `platform_retention_rate`. Add a typed request for `/api/agent/admin/groups`. Remove the agent-wide retention field from update payloads.

- [ ] **Step 2: Replace the group source**

Stop calling the generic `/api/group/` endpoint. Query `/api/agent/admin/groups` only while the drawer is open.

- [ ] **Step 3: Rebuild each group row**

Render the group name plus two labeled percentage inputs: Group Gross Margin and Platform Retention. Remove the top-level Platform Retention Percentage field. Submit a group only when both inputs are valid finite values in `[0,100]`; require at least one complete group rule when enabling an agent.

- [ ] **Step 4: Preserve disable behavior**

Update the disable request to send `enabled: false`, the remark, and an empty `group_margins` array without an agent-wide retention field.

- [ ] **Step 5: Run frontend checks**

Run: `cd web/default && bun run typecheck`

Run: `cd web/default && bunx oxlint -c .oxlintrc.json src/features/agents src/features/users/index.tsx`

Expected: PASS.

### Task 5: Update translations and complete verification

**Files:**
- Modify through script: `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`
- Delete: `docs/superpowers/specs/2026-07-12-agent-sales-settlement-design.md`
- Delete: temporary i18n scripts and generated reports after use

- [ ] **Step 1: Add or revise UI translations through the mandated script**

Use `web/default/scripts/add-missing-keys.mjs` to add compact labels and validation text for per-group retention in every supported locale. Run the script, then `bun run i18n:sync`, and remove temporary scripts/reports.

- [ ] **Step 2: Run cumulative backend verification**

Run: `go test ./model ./service ./controller -count=1`

Expected: PASS.

- [ ] **Step 3: Run cumulative frontend verification**

Run: `cd web/default && bun run typecheck`

Run targeted oxlint, then `bun run build`.

Expected: PASS.

- [ ] **Step 4: Delete the design document as requested**

Delete `docs/superpowers/specs/2026-07-12-agent-sales-settlement-design.md` only after implementation and verification pass.

- [ ] **Step 5: Review and commit only scoped files**

Run `git diff --check`, confirm no `web/classic` changes, preserve unrelated video/deploy/test-results changes, and commit the correction.

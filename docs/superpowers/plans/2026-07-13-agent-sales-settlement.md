# Agent Sales Settlement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add independently configurable agent identities, invitation-period ownership, real-time customer/group earnings statistics, and offline settlement confirmation to the backend and `web/default`.

**Architecture:** Keep system roles and affiliate rewards unchanged. Store compact agent profiles, immutable margin versions, invitation assignment periods, and settlement records; calculate earnings on demand from existing `quota_data`, matching consumption timestamps to assignment and configuration intervals. Expose separate administrator and self-service APIs, then build one shared default-frontend page that adapts to administrator or agent access.

**Tech Stack:** Go 1.22, Gin, GORM v2, SQLite/MySQL/PostgreSQL, testify, React 19, TypeScript, TanStack Router/Query/Table, React Hook Form, Zod, Base UI/shadcn components, Tailwind CSS, i18next, Bun.

---

### Task 1: Add agent persistence models and migrations

**Files:**
- Create: `model/agent.go`
- Modify: `model/main.go`
- Test: `model/agent_test.go`

- [ ] **Step 1: Write failing model tests**

Cover profile creation, immutable complete margin versions, rate validation, duplicate-group rejection, assignment-period open/close behavior, disabling an agent closing current periods, and settlement persistence. Use `require` for setup/fatal assertions and `assert` for value checks.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `go test ./model -run '^TestAgent' -count=1`

Expected: FAIL because agent models and functions do not exist.

- [ ] **Step 3: Implement the model layer**

Create these GORM models with explicit indexes and integer quota amounts:

```go
type AgentProfile struct {
    UserID                int     `gorm:"primaryKey;column:user_id"`
    Enabled               bool    `gorm:"column:enabled;index"`
    PlatformRetentionRate float64 `gorm:"column:platform_retention_rate"`
    Remark                string  `gorm:"type:varchar(255);column:remark"`
    CreatedBy             int     `gorm:"column:created_by"`
    UpdatedBy             int     `gorm:"column:updated_by"`
    CreatedAt             int64   `gorm:"autoCreateTime;column:created_at"`
    UpdatedAt             int64   `gorm:"autoUpdateTime;column:updated_at"`
}

type AgentMarginVersion struct {
    ID                    int     `gorm:"primaryKey;column:id"`
    AgentUserID           int     `gorm:"index:idx_agent_margin_effective,priority:1;column:agent_user_id"`
    PlatformRetentionRate float64 `gorm:"column:platform_retention_rate"`
    EffectiveFrom         int64   `gorm:"index:idx_agent_margin_effective,priority:2;column:effective_from"`
    CreatedBy             int     `gorm:"column:created_by"`
    CreatedAt             int64   `gorm:"autoCreateTime;column:created_at"`
}

type AgentGroupMargin struct {
    ID              int     `gorm:"primaryKey;column:id"`
    VersionID       int     `gorm:"uniqueIndex:idx_agent_version_group,priority:1;column:version_id"`
    Group           string  `gorm:"type:varchar(64);uniqueIndex:idx_agent_version_group,priority:2;column:group_name"`
    GrossMarginRate float64 `gorm:"column:gross_margin_rate"`
}

type AgentCustomerAssignment struct {
    ID             int    `gorm:"primaryKey;column:id"`
    CustomerUserID int    `gorm:"index:idx_agent_customer_period,priority:1;column:customer_user_id"`
    AgentUserID    int    `gorm:"index:idx_agent_assignment_period,priority:1;column:agent_user_id"`
    EffectiveFrom  int64  `gorm:"index:idx_agent_assignment_period,priority:2;column:effective_from"`
    EffectiveTo    *int64 `gorm:"index;column:effective_to"`
    CreatedBy      int    `gorm:"column:created_by"`
    CreatedAt      int64  `gorm:"autoCreateTime;column:created_at"`
}

type AgentSettlement struct {
    ID                     int    `gorm:"primaryKey;column:id"`
    AgentUserID            int    `gorm:"index:idx_agent_settlement_cutoff,priority:1;column:agent_user_id"`
    PeriodStart            int64  `gorm:"column:period_start"`
    PeriodEnd              int64  `gorm:"index:idx_agent_settlement_cutoff,priority:2;column:period_end"`
    ConsumptionQuota       int    `gorm:"column:consumption_quota"`
    GrossProfitQuota       int    `gorm:"column:gross_profit_quota"`
    PlatformRetainedQuota  int    `gorm:"column:platform_retained_quota"`
    AgentEarningsQuota     int    `gorm:"column:agent_earnings_quota"`
    PaymentReference       string `gorm:"type:varchar(255);column:payment_reference"`
    ConfirmedBy            int    `gorm:"column:confirmed_by"`
    ConfirmedAt            int64  `gorm:"column:confirmed_at"`
}
```

Implement transaction-aware functions for saving a complete version, enabling/disabling profiles, opening/closing assignments, listing profiles, and reading effective versions. Validate all rates with finite `[0,1]` bounds and reject duplicate group names.

- [ ] **Step 4: Register migrations**

Add all five models to the existing `AutoMigrate` and table-registration lists in `model/main.go` without database-specific SQL.

- [ ] **Step 5: Run focused model tests**

Run: `go test ./model -run '^TestAgent' -count=1`

Expected: PASS.

### Task 2: Preserve invitation assignment history

**Files:**
- Modify: `controller/user.go`
- Modify: `model/agent.go`
- Test: `controller/user_agent_assignment_test.go`

- [ ] **Step 1: Write failing inviter-update tests**

Test that changing `inviter_id` closes the previous open agent assignment at the update timestamp, opens a new assignment only when the new inviter is enabled as an agent, clearing the inviter closes the period, and a failed user edit rolls back assignment changes.

- [ ] **Step 2: Run the focused controller test and verify failure**

Run: `go test ./controller -run '^TestUpdateUserAgentAssignment' -count=1`

Expected: FAIL because inviter edits do not update assignment history.

- [ ] **Step 3: Integrate assignment updates into the existing transaction**

In `UpdateUser`, after validating the inviter and before `EditWithTx`, call one model function that compares the original and requested inviter. The function closes the current period and opens the new period at one shared timestamp. It must not change normal invitation behavior or affiliate reward fields.

- [ ] **Step 4: Seed current invitees when enabling an agent**

When an administrator first enables or re-enables an agent, open assignment periods at the enable timestamp for current users whose `inviter_id` equals the agent ID. Do not backdate those periods.

- [ ] **Step 5: Run focused tests**

Run: `go test ./controller ./model -run 'AgentAssignment|AgentEnable' -count=1`

Expected: PASS.

### Task 3: Implement real-time agent statistics

**Files:**
- Create: `service/agent_stats.go`
- Test: `service/agent_stats_test.go`

- [ ] **Step 1: Write failing calculation tests**

Seed `quota_data`, assignments, and versioned rules to cover two agents with different rates, a version change, an inviter change, an unconfigured group, deleted customer display data, and exact summary/detail totals.

- [ ] **Step 2: Run the focused service tests and verify failure**

Run: `go test ./service -run '^TestAgentStats' -count=1`

Expected: FAIL because the statistics service does not exist.

- [ ] **Step 3: Implement bounded real-time aggregation**

Define dedicated result types:

```go
type AgentStatsSummary struct {
    CustomerCount          int `json:"customer_count"`
    ConsumptionQuota       int `json:"consumption_quota"`
    GrossProfitQuota       int `json:"gross_profit_quota"`
    PlatformRetainedQuota  int `json:"platform_retained_quota,omitempty"`
    AgentEarningsQuota     int `json:"agent_earnings_quota"`
    SettledQuota           int `json:"settled_quota"`
    PendingQuota           int `json:"pending_quota"`
}

type AgentStatsDetail struct {
    CustomerUserID         int     `json:"customer_user_id"`
    Username               string  `json:"username"`
    Group                  string  `json:"group"`
    ConsumptionQuota       int     `json:"consumption_quota"`
    GrossMarginRate        *float64 `json:"gross_margin_rate"`
    GrossProfitQuota       int     `json:"gross_profit_quota"`
    PlatformRetainedQuota  int     `json:"platform_retained_quota,omitempty"`
    AgentEarningsQuota     int     `json:"agent_earnings_quota"`
    Configured             bool    `json:"configured"`
}
```

Load quota rows within the requested range for customers present in assignment periods, group them by customer/group/time, match assignment and configuration intervals in Go, and use `common.QuotaFromFloatChecked` for both percentage calculations. Saturate safely, clamp no negative amount, and aggregate only to customer/group for output. Add pagination after aggregation and a maximum reporting span for detail requests; cumulative summary may use the agent's first effective profile time through now.

- [ ] **Step 4: Include settlement totals**

Sum confirmed settlements for the agent and compute pending quota as non-negative cumulative earnings minus settled quota. Keep administrator-only retained fields in the internal result so controllers can omit them for agent self responses.

- [ ] **Step 5: Run focused service tests**

Run: `go test ./service -run '^TestAgentStats' -count=1`

Expected: PASS.

### Task 4: Implement settlement preview and confirmation

**Files:**
- Create: `service/agent_settlement.go`
- Test: `service/agent_settlement_test.go`

- [ ] **Step 1: Write failing settlement tests**

Cover first settlement, next non-overlapping settlement, preview/confirmation parity, zero-earnings rejection, duplicate cutoff rejection, disabled-agent historical settlement, and concurrent confirmation protection.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./service -run '^TestAgentSettlement' -count=1`

Expected: FAIL because settlement functions do not exist.

- [ ] **Step 3: Implement preview**

Read the latest confirmed cutoff for the agent. Calculate earnings from the next second through the requested cutoff using the same statistics calculator. Return consumption, gross profit, platform retained, agent earnings, period start, and period end without writing state.

- [ ] **Step 4: Implement transactional confirmation**

Inside `model.DB.Transaction`, lock the agent profile with the shared `lockForUpdate` helper, re-read the latest cutoff, recalculate the preview, reject a non-positive agent amount, and create one immutable `AgentSettlement`. Keep payment reference optional and capped at 255 characters.

- [ ] **Step 5: Run focused settlement tests**

Run: `go test ./service -run '^TestAgentSettlement' -count=1`

Expected: PASS.

### Task 5: Expose administrator and agent APIs

**Files:**
- Create: `dto/agent.go`
- Create: `controller/agent.go`
- Modify: `router/api-router.go`
- Modify: `i18n/keys.go`
- Modify: `i18n/locales/en.yaml`
- Modify: `i18n/locales/zh-CN.yaml`
- Modify: `i18n/locales/zh-TW.yaml`
- Test: `controller/agent_test.go`

- [ ] **Step 1: Write failing API contract tests**

Test admin profile listing/configuration, admin statistics selection, enabled-agent self access, non-agent denial, agent self-response omission of retention fields, settlement preview, confirmation, and target-role management restrictions.

- [ ] **Step 2: Run focused controller tests and verify failure**

Run: `go test ./controller -run '^TestAgentAPI' -count=1`

Expected: FAIL because routes and handlers do not exist.

- [ ] **Step 3: Define request/response DTOs**

Use pointer scalar fields for optional JSON values. Configuration requests contain `enabled`, `platform_retention_rate`, `remark`, and a complete array of `{group, gross_margin_rate}` rules. Statistics requests validate timestamps, pagination, and optional search. Settlement requests contain cutoff and optional payment reference.

- [ ] **Step 4: Implement handlers and routes**

Create administrator routes under `/api/agent/admin` with `AdminAuth`, and self routes under `/api/agent/self` with `UserAuth`. Keep self handlers pinned to `c.GetInt("id")`; never accept another agent ID. Add backend i18n keys for invalid rates, no configured groups, not-an-agent, no unsettled earnings, invalid cutoff, and duplicate settlement.

- [ ] **Step 5: Add agent state to authenticated self and user-list responses**

Expose `agent_enabled` on `/api/user/self` and attach agent-enabled state to paginated administrator user-list results using one bulk profile query rather than per-row requests.

- [ ] **Step 6: Run focused API tests**

Run: `go test ./controller ./service ./model -run 'Agent' -count=1`

Expected: PASS.

### Task 6: Add default-frontend agent types, API, and route authorization

**Files:**
- Create: `web/default/src/features/agents/types.ts`
- Create: `web/default/src/features/agents/api.ts`
- Create: `web/default/src/features/agents/constants.ts`
- Create: `web/default/src/routes/_authenticated/agent-users/index.tsx`
- Modify: `web/default/src/stores/auth-store.ts`
- Modify: `web/default/src/hooks/use-sidebar-data.ts`

- [ ] **Step 1: Add typed schemas and API functions**

Define Zod schemas for agent profiles, margin rules, summary, customer-group rows, settlement previews, settlement history, and paginated responses. Add React Query-friendly API functions that select admin or self endpoints based on caller access.

- [ ] **Step 2: Add the authenticated route**

Permit the route when `role >= ROLE.ADMIN` or `auth.user.agent_enabled === true`; otherwise redirect to `/403`. Validate URL search state for selected agent, page, page size, customer search, group, start time, and end time.

- [ ] **Step 3: Add role-aware navigation**

Administrators see Agent Users in the administrator section. Enabled agents below administrator role see Agent Users in the regular section. Reuse the existing sidebar item structure and icon library.

- [ ] **Step 4: Run type checking**

Run: `cd web/default && bun run typecheck`

Expected: PASS.

### Task 7: Add agent controls to the user list

**Files:**
- Modify: `web/default/src/features/users/types.ts`
- Modify: `web/default/src/features/users/components/users-columns.tsx`
- Modify: `web/default/src/features/users/components/users-provider.tsx`
- Create: `web/default/src/features/agents/components/agent-settings-drawer.tsx`
- Create: `web/default/src/features/agents/components/agent-disable-dialog.tsx`
- Create: `web/default/src/features/agents/lib/agent-form.ts`

- [ ] **Step 1: Extend user data and provider state**

Add `agent_enabled` to the user schema. Add provider dialog state for configuring and disabling the selected user's agent identity without changing existing user edit behavior.

- [ ] **Step 2: Add the Agent table column**

Render the existing `Switch` component. Switching on opens the settings drawer and does not mutate until save. Switching off opens the confirmation dialog. Administrators cannot manage same/higher-role targets, matching backend rules.

- [ ] **Step 3: Build the settings form by reusing existing patterns**

Reuse `Sheet`, drawer-layout classes, React Hook Form, Zod, existing group query, `Input`, `Textarea`, and toast handling. The form edits retention percentage, selected group gross-margin percentages, and remark. Save a complete rule version and invalidate users/agent queries.

- [ ] **Step 4: Build disable confirmation**

Reuse the project's confirmation dialog pattern and explain that historical statistics and settlements remain available.

- [ ] **Step 5: Run targeted checks**

Run: `cd web/default && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/users src/features/agents`

Expected: PASS with no errors in changed feature files.

### Task 8: Build the shared Agent Users page

**Files:**
- Create: `web/default/src/features/agents/index.tsx`
- Create: `web/default/src/features/agents/components/agent-selector.tsx`
- Create: `web/default/src/features/agents/components/agent-summary-cards.tsx`
- Create: `web/default/src/features/agents/components/agent-earnings-table.tsx`
- Create: `web/default/src/features/agents/components/agent-settlements-table.tsx`
- Create: `web/default/src/features/agents/components/agent-settlement-dialog.tsx`
- Create: `web/default/src/features/agents/components/agent-filters.tsx`

- [ ] **Step 1: Build the role-aware page shell**

Reuse `SectionPageLayout`. Administrators receive an agent selector and configuration/settlement actions; agents load their own profile directly. Use URL search parameters for filters and pagination.

- [ ] **Step 2: Add summary cards**

Reuse dashboard `StatCard` for customer count, consumption, cumulative earnings, settled amount, and pending amount. Add gross profit and platform retained cards only for administrators.

- [ ] **Step 3: Add earnings detail**

Reuse TanStack table and existing data-table controls. Render customer, group, consumption, margin rate, gross profit, agent earnings, and configuration status. Render platform retained only for administrators. Support customer search, group filter, date range, pagination, loading, error, and empty states.

- [ ] **Step 4: Add settlement history and confirmation**

Reuse `Tabs` for Earnings Detail and Settlement History. The administrator-only dialog previews the server-calculated settlement, accepts cutoff and payment reference, then confirms and invalidates all agent queries.

- [ ] **Step 5: Run frontend checks**

Run: `cd web/default && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/agents src/routes/_authenticated/agent-users/index.tsx src/hooks/use-sidebar-data.ts`

Expected: PASS.

### Task 9: Add frontend internationalization

**Files:**
- Create temporarily: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/en.json`
- Modify through script: `web/default/src/i18n/locales/zh.json`
- Modify through script: `web/default/src/i18n/locales/zh-TW.json`
- Modify through script: `web/default/src/i18n/locales/fr.json`
- Modify through script: `web/default/src/i18n/locales/ja.json`
- Modify through script: `web/default/src/i18n/locales/ru.json`
- Modify through script: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Add all new UI keys through the sanctioned script**

Populate complete translations for agent identity, settings, margins, summary cards, unconfigured states, settlement preview/history, confirmation text, filters, errors, and navigation across every existing locale.

- [ ] **Step 2: Apply, normalize, and verify**

Run: `cd web/default && node scripts/add-missing-keys.mjs && bun run i18n:sync`

Expected: all locale files contain the keys and the sync report has zero missing keys.

- [ ] **Step 3: Remove the temporary script**

Delete `web/default/scripts/add-missing-keys.mjs` after the locale files are updated.

- [ ] **Step 4: Run frontend validation**

Run: `cd web/default && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/agents src/features/users src/routes/_authenticated/agent-users/index.tsx src/hooks/use-sidebar-data.ts`

Expected: PASS.

### Task 10: Final verification and scoped commit

**Files:**
- Review all agent feature files and migrations
- Preserve existing unrelated `controller/video_proxy.go`, `controller/video_proxy_test.go`, deleted video-proxy design document, `deploy/`, and `test-results/` changes

- [ ] **Step 1: Run backend verification**

Run: `go test ./model ./service ./controller -count=1`

Expected: PASS.

- [ ] **Step 2: Run frontend verification**

Run: `cd web/default && bun run typecheck`

Run targeted lint over every modified TypeScript/TSX file.

Expected: PASS. If whole-repository lint still reports pre-existing unrelated errors, report them separately without editing unrelated files.

- [ ] **Step 3: Inspect the final diff**

Run: `git diff --check` and review `git status --short` plus the staged diff. Confirm no `web/classic` file and no unrelated video/deploy/test-results change is staged.

- [ ] **Step 4: Commit the scoped implementation**

Stage only agent feature backend/frontend code, migrations, tests, translations, and this plan if retained. Commit with `feat: add agent sales settlement`.

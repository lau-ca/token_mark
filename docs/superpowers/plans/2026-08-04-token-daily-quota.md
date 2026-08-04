# Token Daily Quota Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional fixed daily API-key quota that resets at local midnight, does not roll over, and remains safe with concurrent requests, Redis caching, batch quota updates, and cross-day settlement.

**Architecture:** Daily quota remains a token-level concern. The database is authoritative; request-time reset provides correctness, a master-node sweep refreshes inactive keys, and Redis is refreshed from committed rows. Daily reservations use conditional direct database updates and carry the reset-period identifier through synchronous and asynchronous billing so old-period adjustments cannot change a new day's allowance.

**Tech Stack:** Go 1.22, Gin, GORM v2, SQLite/MySQL/PostgreSQL, Redis hash cache, React 19, TypeScript, React Hook Form, Zod, Base UI, Bun.

---

### Task 1: Daily quota model and reset invariants

**Files:**
- Create: `model/token_daily_quota.go`
- Create: `model/token_daily_quota_test.go`
- Modify: `model/token.go`

- [ ] **Step 1: Write model tests for daily reset and reservation behavior**

Cover exact observable contracts with deterministic SQLite tests:

```go
func TestEnsureTokenDailyQuotaResetsDueToken(t *testing.T)
func TestEnsureTokenDailyQuotaPreservesDisabledStatus(t *testing.T)
func TestReserveTokenQuotaDoesNotExceedDailyAllowance(t *testing.T)
func TestAdjustTokenQuotaSkipsPreviousPeriod(t *testing.T)
func TestAdjustTokenQuotaCapsSamePeriodRefund(t *testing.T)
func TestResetDueTokenDailyQuotasResetsInactiveKeys(t *testing.T)
```

Each test seeds a `Token` with explicit `DailyQuotaEnabled`, `DailyQuota`, `DailyQuotaNextReset`, `RemainQuota`, `UsedQuota`, and `Status`, then asserts the committed row.

- [ ] **Step 2: Run the focused tests and verify failure**

Run:

```bash
go test ./model -run 'Test(EnsureTokenDailyQuota|ReserveTokenQuota|AdjustTokenQuota|ResetDueTokenDailyQuotas)' -count=1
```

Expected: compilation fails because the fields and model functions do not exist.

- [ ] **Step 3: Add token fields**

Add to `model.Token`:

```go
DailyQuotaEnabled   bool  `json:"daily_quota_enabled"`
DailyQuota          int   `json:"daily_quota" gorm:"default:0"`
DailyQuotaNextReset int64 `json:"daily_quota_next_reset" gorm:"bigint;default:0;index"`
```

Add the three fields to the explicit `Token.Update()` select list.

- [ ] **Step 4: Implement the daily quota domain functions**

Create these stable model APIs in `model/token_daily_quota.go`:

```go
var ErrTokenQuotaInsufficient = errors.New("token quota is not enough")

func NextTokenDailyQuotaReset(now time.Time) int64
func EnsureTokenDailyQuota(token *Token) error
func ReserveTokenQuota(token *Token, quota int) (int64, error)
func AdjustTokenQuotaForPeriod(tokenId int, key string, delta int, period int64) error
func ResetDueTokenDailyQuotas(limit int) (int, error)
```

Implementation requirements:

- `NextTokenDailyQuotaReset` returns the next `00:00` in `time.Local`.
- `EnsureTokenDailyQuota` returns immediately for non-daily or not-due keys; otherwise it reloads and locks the row with `lockForUpdate(tx)`, resets `remain_quota`, `used_quota`, status, and next reset, commits, updates the caller's struct, and refreshes Redis synchronously.
- `ReserveTokenQuota` uses the existing path for normal keys. For daily keys it first ensures reset, then performs a conditional `Updates` with `remain_quota >= quota` and the captured `daily_quota_next_reset`; zero affected rows returns `ErrTokenQuotaInsufficient`.
- `AdjustTokenQuotaForPeriod` uses existing increase/decrease functions when `period == 0`. For a daily period it locks the row, ensures any due reset, skips adjustment if the current period differs, applies a charge only when enough current allowance exists, and caps refunds at `daily_quota` while keeping `used_quota >= 0`.
- `ResetDueTokenDailyQuotas` selects due rows in bounded batches and calls the idempotent reset path.
- Daily paths bypass `BatchUpdateTypeTokenQuota` and refresh Redis from the database row after commit.

- [ ] **Step 5: Run model tests**

Run the focused command from Step 2. Expected: PASS.

---

### Task 2: Token API normalization and authentication refresh

**Files:**
- Modify: `controller/token.go`
- Modify: `middleware/auth.go`
- Modify: `constant/context_key.go`
- Modify: `relay/common/relay_info.go`
- Create: `controller/token_daily_quota_test.go`

- [ ] **Step 1: Write controller normalization tests**

Test the shared validation/normalization function with table cases:

```go
func TestNormalizeTokenQuotaSettings(t *testing.T)
```

Cases must assert:

- normal quota remains unchanged;
- unlimited clears daily fields;
- daily plus unlimited is rejected;
- negative and over-limit daily quota are rejected;
- enabling or changing daily quota sets `remain_quota = daily_quota`, `used_quota = 0`, and next local midnight;
- unchanged daily configuration preserves the active period balance.

- [ ] **Step 2: Implement shared request normalization**

In `controller/token.go`, add one shared function used by both add and update:

```go
func normalizeTokenQuotaSettings(input *model.Token, current *model.Token) error
```

Use the existing maximum quota formula. On create, normalize daily configuration before building `cleanToken`. On update, compare with the persisted token so only enabling daily mode or changing its configured amount starts a fresh allowance.

- [ ] **Step 3: Persist and return daily fields**

Copy daily fields into `cleanToken` in `AddToken`, copy normalized fields in `UpdateToken`, and preserve them in status-only updates. Ensure `GetTokenUsage` calls `model.EnsureTokenDailyQuota` before constructing its response.

- [ ] **Step 4: Refresh overdue daily quota during authentication**

Call `model.EnsureTokenDailyQuota(token)` immediately after `GetTokenByKey` in `ValidateUserToken`, before status and remaining-quota rejection.

Add context keys:

```go
ContextKeyTokenDailyQuotaEnabled   ContextKey = "token_daily_quota_enabled"
ContextKeyTokenDailyQuotaNextReset ContextKey = "token_daily_quota_next_reset"
```

Set them in `SetupContextForToken`, then initialize matching `RelayInfo` fields:

```go
TokenDailyQuotaEnabled   bool
TokenDailyQuotaNextReset int64
```

- [ ] **Step 5: Run controller and model tests**

Run:

```bash
go test ./controller ./model -run 'Test(NormalizeTokenQuotaSettings|EnsureTokenDailyQuota|ReserveTokenQuota|AdjustTokenQuota|ResetDueTokenDailyQuotas)' -count=1
```

Expected: PASS.

---

### Task 3: Period-aware synchronous billing

**Files:**
- Modify: `service/quota.go`
- Modify: `service/billing_session.go`
- Modify: `service/task_billing.go`
- Modify: `model/task.go`
- Modify: `controller/relay.go`
- Modify: `service/task_billing_test.go`

- [ ] **Step 1: Add billing regression tests**

Add tests proving:

```go
func TestBillingSessionDailyQuotaDisablesTrustBypass(t *testing.T)
func TestBillingSessionDailyQuotaRefundUsesReservedPeriod(t *testing.T)
func TestTaskAdjustTokenQuotaSkipsExpiredDailyPeriod(t *testing.T)
```

The tests must assert database balances, not helper internals.

- [ ] **Step 2: Reserve through the daily-aware model API**

Replace the read/check/decrement sequence in `PreConsumeTokenQuota` with:

```go
period, err := model.ReserveTokenQuota(token, quota)
if err != nil {
    return err
}
relayInfo.TokenDailyQuotaNextReset = period
relayInfo.TokenDailyQuotaEnabled = period > 0
```

Keep the existing formatted insufficient-quota message by translating `model.ErrTokenQuotaInsufficient` at the service boundary.

- [ ] **Step 3: Route all session adjustments through the captured period**

In `BillingSession.Settle`, `Refund`, failed-funding rollback, and extra reservation, call:

```go
model.AdjustTokenQuotaForPeriod(tokenId, tokenKey, delta, dailyQuotaPeriod)
```

Use positive `delta` for additional consumption and negative `delta` for refunds. Copy the period into asynchronous refund closures.

- [ ] **Step 4: Disable trust bypass for daily keys**

Add an early return in `BillingSession.shouldTrust`:

```go
if s.relayInfo.TokenDailyQuotaEnabled {
    return false
}
```

- [ ] **Step 5: Persist the task period**

Add to `TaskPrivateData`:

```go
TokenDailyQuotaNextReset int64 `json:"token_daily_quota_next_reset,omitempty"`
```

Set it when creating a task in `controller/relay.go`. Pass it from `taskAdjustTokenQuota` into `AdjustTokenQuotaForPeriod` so cross-day task refunds and recalculations cannot alter the new day's allowance.

- [ ] **Step 6: Update legacy post-consume paths**

Replace direct `IncreaseTokenQuota`/`DecreaseTokenQuota` calls in `PostConsumeQuota` with `AdjustTokenQuotaForPeriod` using the `RelayInfo` period. This covers older relay paths that do not use `BillingSession` consistently.

- [ ] **Step 7: Run billing tests**

Run:

```bash
go test ./service -run 'Test(BillingSessionDailyQuota|TaskAdjustTokenQuota|RefundTaskQuota|RecalculateTaskQuota)' -count=1
```

Expected: PASS.

---

### Task 4: Background reset maintenance

**Files:**
- Create: `service/token_daily_quota_task.go`
- Create: `service/token_daily_quota_task_test.go`
- Modify: `main.go`

- [ ] **Step 1: Write the maintenance test**

Test one run with multiple due keys and one future key. Assert that due keys reset and the future key is unchanged.

- [ ] **Step 2: Implement the master-node task**

Follow the subscription reset task pattern with separate state:

```go
const tokenDailyQuotaResetTickInterval = time.Minute
const tokenDailyQuotaResetBatchSize = 300

func StartTokenDailyQuotaResetTask()
func runTokenDailyQuotaResetOnce()
```

Run only on `common.IsMasterNode`, prevent overlapping runs with `atomic.Bool`, loop through bounded batches, and log failures without stopping request-time reset fallback.

- [ ] **Step 3: Start the task from main**

Call `service.StartTokenDailyQuotaResetTask()` adjacent to subscription quota maintenance.

- [ ] **Step 4: Run maintenance tests**

Run:

```bash
go test ./service -run 'TestRunTokenDailyQuotaResetOnce' -count=1
```

Expected: PASS.

---

### Task 5: Frontend quota mode and API contract

**Files:**
- Modify: `web/default/src/features/keys/types.ts`
- Modify: `web/default/src/features/keys/lib/api-key-form.ts`
- Create: `web/default/src/features/keys/lib/api-key-form.test.ts`
- Modify: `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-columns.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-table.tsx`

- [ ] **Step 1: Write transformation tests**

Use `node:test` and `node:assert/strict` to verify:

```ts
test('transforms total quota mode')
test('transforms daily quota mode')
test('transforms unlimited quota mode')
test('loads daily API keys into daily quota mode')
```

- [ ] **Step 2: Extend API types**

Add response fields to `apiKeySchema` and request fields to `ApiKeyFormData`:

```ts
daily_quota_enabled: boolean
daily_quota: number
daily_quota_next_reset: number
```

Use frontend-only form state:

```ts
quota_mode: z.enum(['total', 'daily', 'unlimited'])
quota_dollars: z.number().optional()
```

Transform modes so daily sends both `daily_quota` and the initial `remain_quota`, total sends only lifetime quota, and unlimited clears both quota values.

- [ ] **Step 3: Replace conflicting quota controls with one mode selector**

In the quota section, render a `RadioGroup` with Total quota, Daily quota, and Unlimited quota. Render the single amount input only for total or daily mode. For daily mode, show the description that the allowance restores at 00:00 and unused quota does not roll over.

- [ ] **Step 4: Mark daily keys in desktop and mobile lists**

Show the existing `StatusBadge` with label `Daily` beside the current/total quota presentation when `daily_quota_enabled` is true. Do not add a new table column.

- [ ] **Step 5: Run focused frontend tests and type checking**

Run:

```bash
cd web/default
bun test src/features/keys/lib/api-key-form.test.ts
bun run typecheck
```

Expected: PASS.

---

### Task 6: Internationalization and final verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify: `web/default/src/i18n/locales/zh-TW.json`

- [ ] **Step 1: Add locale strings without overwriting existing user changes**

Add translations for these exact source keys:

```text
Total quota
Daily quota
Restores to the configured quota every day at 00:00; unused quota does not roll over
Quota mode
```

Reuse the existing `Daily`, `Unlimited`, and quota validation keys where possible.

- [ ] **Step 2: Format and lint changed files**

Run targeted formatting, then lint the frontend. Do not run a destructive rewrite across unrelated files in the dirty worktree.

```bash
gofmt -w controller/token.go controller/token_daily_quota_test.go model/token.go model/token_daily_quota.go model/token_daily_quota_test.go middleware/auth.go constant/context_key.go relay/common/relay_info.go service/quota.go service/billing_session.go service/task_billing.go service/token_daily_quota_task.go service/token_daily_quota_task_test.go model/task.go controller/relay.go main.go
cd web/default
bun run format -- src/features/keys/types.ts src/features/keys/lib/api-key-form.ts src/features/keys/lib/api-key-form.test.ts src/features/keys/components/api-keys-mutate-drawer.tsx src/features/keys/components/api-keys-columns.tsx src/features/keys/components/api-keys-table.tsx
bun run lint
```

- [ ] **Step 3: Run backend regression tests**

Run:

```bash
go test ./model ./controller ./service -count=1
```

Expected: PASS.

- [ ] **Step 4: Run frontend verification**

Run:

```bash
cd web/default
bun test src/features/keys/lib/api-key-form.test.ts
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 5: Review the final diff**

Confirm:

- protected project identity and metadata are untouched;
- unrelated dirty files are not staged or rewritten;
- existing tokens default to daily quota disabled;
- no direct `encoding/json` marshal/unmarshal calls were introduced;
- database operations remain compatible with SQLite, MySQL, and PostgreSQL;
- daily quota paths never use delayed token batching;
- no browser or end-to-end testing was added.

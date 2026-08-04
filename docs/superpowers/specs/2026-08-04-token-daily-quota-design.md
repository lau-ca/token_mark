# Token Daily Quota Design

## Goal

Add an optional fixed daily quota mode to API keys so employee keys can spend at most a configured amount per local calendar day. Existing keys keep their current lifetime quota behavior unless daily quota is explicitly enabled.

## Confirmed Product Rules

- A key has either the existing lifetime quota mode or the new daily quota mode.
- Daily quota is a fixed amount configured on the key.
- The allowance resets at 00:00 in the server's configured local timezone.
- Unused allowance does not roll over.
- Unlimited quota and daily quota are mutually exclusive.
- Enabling daily quota starts a fresh daily allowance immediately and schedules the next reset for the next local midnight.
- Disabling daily quota keeps the currently displayed remaining quota as the key's lifetime remaining quota.
- Existing keys default to daily quota disabled.

## Approaches Considered

### Scheduled database reset only

A background task could set every due key's `remain_quota` back to its configured daily amount. This is small, but it is unsafe when the scheduler is delayed, Redis still contains the old balance, or batched quota deductions arrive after the reset.

### Request-time reset only

Each authenticated request could reset an overdue key before checking its balance. This keeps active keys working even if a scheduler is unavailable, but inactive keys would show stale quota in the management UI until used.

### Hybrid reset with period-aware billing

Use an idempotent request-time reset as the correctness path and a background sweep as maintenance. Daily-key deductions bypass delayed token quota batching, use a conditional database update, and refresh Redis from the committed database row. Billing adjustments carry the daily period they belong to so a refund from the previous day cannot increase the new day's allowance.

This is the recommended approach because it keeps the feature scoped to token quota while protecting the daily limit under scheduler delays, cache staleness, and concurrent requests.

## Data Model

Add the following fields to `model.Token`:

- `DailyQuotaEnabled bool`: selects fixed daily quota behavior.
- `DailyQuota int`: configured allowance restored at each reset.
- `DailyQuotaNextReset int64`: Unix timestamp of the next local midnight; indexed for the maintenance sweep.

`remain_quota` remains the currently available amount. `used_quota` represents consumption in the current daily period for daily keys and retains its existing lifetime meaning for normal keys.

No separate period-history table is required. Usage history continues to come from consume logs.

## Reset Semantics

The model layer owns daily reset behavior.

When a daily key is due:

1. Lock or conditionally claim the due token row using cross-database-compatible GORM operations.
2. Set `remain_quota = daily_quota` and `used_quota = 0`.
3. Change status from exhausted to enabled, while preserving explicitly disabled or expired states.
4. Advance `daily_quota_next_reset` to the next local midnight. If multiple days were missed, advance directly to the next future midnight rather than applying multiple grants.
5. Commit the transaction and synchronously refresh the Redis token hash.

The reset operation must be idempotent and safe when request-time reset and background maintenance run concurrently.

## Quota Reservation and Concurrency

Normal keys retain their existing quota update path. Daily keys use a dedicated direct database path and do not enqueue token quota changes in the in-process batch updater.

Before reserving daily quota, the model performs the due reset if necessary. Reservation then uses a conditional update equivalent to:

```sql
UPDATE tokens
SET remain_quota = remain_quota - ?,
    used_quota = used_quota + ?,
    accessed_time = ?
WHERE id = ?
  AND daily_quota_enabled = true
  AND daily_quota_next_reset = ?
  AND remain_quota >= ?
```

The affected-row count is the source of truth. Zero affected rows means the allowance is insufficient or the period changed, so the caller reloads once and either retries the new period or returns insufficient token quota.

The existing trust-quota pre-consume bypass is disabled for daily keys. A security limit is only meaningful if every paid request reserves token quota before upstream work begins.

## Refunds, Settlement, and Cross-Midnight Requests

`RelayInfo` records the daily quota period identifier, represented by the key's `daily_quota_next_reset` value at reservation time. `TaskPrivateData` stores the same value for asynchronous task settlement.

Adjustments use the following rules:

- Normal keys keep the existing increase/decrease behavior.
- A daily-key adjustment is applied only while the token is still in the reservation's period.
- If the token has advanced to a new day, refunds and settlement differences from the old period do not modify the new day's token allowance.
- Funding-source accounting and consume/refund logs keep their existing behavior; only the token-level employee limit is period-scoped.

Async tasks already force full pre-consumption. Skipping a late old-period token refund is the conservative security behavior and prevents yesterday's task from granting additional allowance today.

## Redis and Batch Updates

- Daily reset and daily quota reservation commit to the primary database before updating Redis.
- Redis is refreshed from the committed token row rather than adjusted optimistically.
- Daily keys bypass `BatchUpdateTypeTokenQuota` so a delayed in-memory delta cannot cross a daily boundary.
- Enabling daily quota establishes a fresh allowance immediately. Existing pending lifetime-quota deltas are not allowed to mutate the new daily period.

## API and Validation

The existing create and update token endpoints accept and return:

- `daily_quota_enabled`
- `daily_quota`
- `daily_quota_next_reset` as read-only response state

Validation rules:

- Daily quota must be between zero and the existing maximum token quota.
- Daily quota cannot be enabled together with unlimited quota.
- When daily quota is enabled, `remain_quota` is normalized to `daily_quota` on create and on a transition from disabled to enabled.
- Editing the configured daily amount during an active period starts a fresh allowance at the new amount immediately.
- Batch key creation applies the same daily quota configuration to every created key.

## Frontend

The API key drawer keeps the existing quota section and adds a quota mode choice:

- Total quota
- Daily quota

The existing unlimited quota switch remains available, but enabling it clears daily mode. In daily mode, the amount label and description explicitly state that the value is restored every day at 00:00 and unused quota does not roll over.

The key list continues to display current remaining and used quota. Daily keys also receive a compact `Daily` indicator so administrators can distinguish the quota semantics.

All user-facing strings are added to every supported default-theme locale through the existing i18n workflow.

## Error Handling

- A failed reset leaves the previous database and cache state intact and rejects the request rather than granting untracked usage.
- A conditional reservation failure returns the existing insufficient token quota error contract.
- Redis refresh failure is logged; database state remains authoritative and the next token read falls back to or refreshes from the database.
- Background sweep failures are logged and retried on the next interval. Request-time reset remains the correctness fallback.

## Testing

Backend tests protect these behaviors:

- Existing tokens remain unchanged after migration.
- Creating and editing daily keys normalizes quota and next-reset state.
- Due reset restores exactly the configured amount with no rollover.
- Exhausted daily keys become enabled after reset; disabled and expired keys do not.
- Two concurrent reservations cannot jointly exceed the daily allowance.
- Daily reservations bypass the token batch updater.
- A previous-period refund does not increase the current day's allowance.
- A same-period refund restores allowance without exceeding the configured daily amount.
- The reset calculation advances correctly across local calendar days.

Frontend tests cover schema validation and payload/default transformations for total, daily, and unlimited quota modes.

Verification includes focused Go tests, frontend unit tests, TypeScript type checking, linting of changed frontend files, and a production frontend build. Browser end-to-end testing is outside this implementation unless requested separately.

## Scope Boundaries

This feature does not add weekly, monthly, custom-interval, per-key timezone, rollover, daily history, or automatic employee account provisioning. It does not change user wallet balance, subscription entitlement, channel billing, model pricing, or consume-log calculations.

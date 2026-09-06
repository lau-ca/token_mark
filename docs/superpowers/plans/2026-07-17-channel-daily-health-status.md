# Channel Daily Health Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persisted Beijing-day channel health snapshot, refresh it with channel balance, and display left-aligned usage, balance, status, error rate, and call count in the channel list.

**Architecture:** Aggregate `LogTypeConsume` and `LogTypeError` from `model.LOG_DB` by channel for the current UTC+8 day, classify the result, and persist a compact snapshot on `channels`. Manual, bulk, and scheduled balance refreshes update health independently. The frontend renders fixed-width stacked rows with tabular numerals and tooltips.

**Tech Stack:** Go 1.22, Gin, GORM v2, React 19, TypeScript, React Hook Form, Base UI, Tailwind CSS, i18next, Bun.

---

### Task 1: Persist and Aggregate Channel Health

**Files:**
- Modify: `model/channel.go`
- Create: `model/channel_health.go`
- Create: `model/channel_health_test.go`
- Modify: `controller/channel_authz.go`

- [ ] Add read-only channel fields `health_status`, `health_error_rate`, `health_success_count`, `health_error_count`, `health_total_count`, `health_date`, and `health_updated_time`.
- [ ] Add a UTC+8 day-range helper using `time.FixedZone("Asia/Shanghai", 8*60*60)`.
- [ ] Add `GetChannelHealthCounts(channelIDs, start, end)` using one grouped conditional-aggregation query against `LOG_DB` and types 2/5.
- [ ] Add `UpdateHealthSnapshot` with explicit selected columns so zero values persist.
- [ ] Test Beijing midnight boundaries, channel isolation, type filtering, batch results, and zero persistence.
- [ ] Run `go test ./model -run 'TestChannelHealth' -count=1` and commit.

Core aggregation shape:

```go
type ChannelHealthCounts struct {
    ChannelID   int   `gorm:"column:channel_id"`
    SuccessCount int64 `gorm:"column:success_count"`
    ErrorCount   int64 `gorm:"column:error_count"`
}
```

### Task 2: Refresh Health with Balance

**Files:**
- Modify: `controller/channel-billing.go`
- Create: `controller/channel_health_test.go`

- [ ] Add status constants `unknown`, `healthy`, `warning`, and `critical`.
- [ ] Classify fewer than 20 attempts as unknown, `<2%` healthy, `<10%` warning, and otherwise critical.
- [ ] Add a batch health refresh that queries all relevant channels once and persists every snapshot.
- [ ] Manual refresh runs balance and health independently and returns combined errors without discarding a successful half.
- [ ] Bulk and scheduled refresh update health once per trigger, then retain the current sequential balance behavior.
- [ ] Test exact 2% and 10% boundaries, disabled consume logs, independent errors, and total calls equal success plus errors.
- [ ] Run focused controller tests and commit.

### Task 3: Render Aligned Usage, Balance, and Health Columns

**Files:**
- Modify: `web/default/src/features/channels/types.ts`
- Modify: `web/default/src/features/channels/lib/channel-utils.ts`
- Create: `web/default/src/features/channels/lib/channel-health.test.mjs`
- Modify: `web/default/src/features/channels/components/channels-columns.tsx`
- Modify: `web/default/src/features/channels/components/channel-card.tsx`
- Modify: `web/default/src/features/channels/lib/channel-actions.ts`

- [ ] Add the health snapshot fields to the channel schema.
- [ ] Add Beijing-date, current-status, percentage, and color helpers with tests.
- [ ] Restyle usage/balance into two left-aligned fixed-label rows using `tabular-nums`.
- [ ] Add a fixed-width health column immediately after balance with three left-aligned rows: status, error rate, and calls.
- [ ] Show success/error counts, date, and update time in a tooltip.
- [ ] Render the same health cell on mobile cards.
- [ ] Always invalidate channel data after manual refresh, including partial failure.
- [ ] Run Bun tests, typecheck, targeted oxlint, and commit.

Target cell structure:

```tsx
<div className='grid min-w-[10rem] gap-1.5 text-left'>
  <div className='grid grid-cols-[3.5rem_1fr] items-center gap-2'>
    <span className='text-muted-foreground'>Status</span>
    <span className='tabular-nums'>Healthy</span>
  </div>
</div>
```

### Task 4: Add Complete Translations

**Files:**
- Temporarily create: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: all locale JSON files under `web/default/src/i18n/locales/`

- [ ] Add translations for `Usage / Balance`, `Status (error rate)`, `Status`, `Error rate`, `Calls`, `Healthy`, `Warning`, `Critical`, `Insufficient samples`, `Pending refresh`, `Successes`, `Errors`, and `Reporting date`.
- [ ] Apply translations through the required script, run `bun run i18n:sync`, verify no new missing keys, and delete the temporary script.
- [ ] Commit locale changes.

### Task 5: Verify, Package, and Deploy Master Manually

**Files:**
- Verify all feature files and committed source.

- [ ] Run `go test ./controller ./model`.
- [ ] Run frontend unit tests, `bun run typecheck`, targeted oxlint, `bun run build`, and `bun run i18n:sync`.
- [ ] Inspect credential safety, diff whitespace, and unrelated worktree changes.
- [ ] Build a clean `linux/amd64` image from `git archive HEAD` without deployment scripts.
- [ ] Inspect and back up `/root/gateway/master/docker-compose.yml` and the running master image on `157.230.213.186`.
- [ ] Upload to a temporary filename, verify SHA256, load the image, and recreate only `new-api-master`.
- [ ] Verify health, restart count, new database columns, localhost/public endpoints, and unchanged worker/CPA/nginx container IDs.
- [ ] Remove temporary image archives while retaining the remote backup.

### Task 6: Separate Health Refresh from Balance Refresh

**Files:**
- Modify: `controller/channel-billing.go`
- Modify: `router/channel-router.go`
- Modify: `web/default/src/features/channels/api.ts`
- Modify: `web/default/src/features/channels/lib/channel-actions.ts`
- Modify: `web/default/src/features/channels/components/channels-columns.tsx`
- Modify: `web/default/src/features/channels/components/channels-primary-buttons.tsx`
- Modify through script: `web/default/src/i18n/locales/*.json`

- [ ] Remove health refresh calls from the single-channel and all-channel balance handlers.
- [ ] Add `GET /api/channel/update_health/:id` for one channel and `GET /api/channel/update_health` for all channels.
- [ ] Keep scheduled balance and health refreshes in the same configured cycle while executing them as separate operations with independent error handling.
- [ ] Add frontend API/action functions for single and bulk health refresh.
- [ ] Make the status value clickable and show its own updating state without invoking the balance API.
- [ ] Add an `Update All Statuses` menu item next to `Update All Balances`.
- [ ] Add complete translations through `scripts/add-missing-keys.mjs`, run `bun run i18n:sync`, and delete the temporary script.
- [ ] Run backend tests, frontend tests, typecheck, targeted lint, and production build.
- [ ] Commit, build from `git archive HEAD`, and manually deploy only `new-api-master`.

# Quota Forecast Display Refinement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the full predicted exhaustion time on the dashboard and add a page-native forecast badge to `/usage-logs/common` without delaying either list.

**Architecture:** Extend the existing shared quota forecast display with a compact stats variant and centralize the self-forecast React Query key/stale time. The dashboard will reserve enough width for the exact timestamp, while common usage logs will fetch the same cached self forecast independently and render it only in self scope.

**Tech Stack:** React 19, TypeScript, TanStack Query, Tailwind CSS, i18next, Bun, Rsbuild

---

### Task 1: Centralize the self-forecast query contract

**Files:**
- Modify: `web/default/src/features/quota-forecast/api.ts`
- Modify: `web/default/src/features/dashboard/components/overview/summary-cards.tsx`

- [ ] **Step 1: Export the stable query key and stale time**

Add the following exports beside the existing API functions:

```ts
export const SELF_QUOTA_FORECAST_QUERY_KEY = [
  'quota-forecast',
  'self',
] as const
export const QUOTA_FORECAST_STALE_TIME = 5 * 60 * 1000
```

- [ ] **Step 2: Reuse the contract on the dashboard**

Import the two constants and replace the inline query key and stale time:

```ts
const forecastQuery = useQuery({
  queryKey: SELF_QUOTA_FORECAST_QUERY_KEY,
  queryFn: getSelfQuotaForecast,
  staleTime: QUOTA_FORECAST_STALE_TIME,
  retry: 1,
})
```

- [ ] **Step 3: Run typecheck**

Run: `bun run typecheck`

Expected: exit code 0.

### Task 2: Make the dashboard timestamp non-truncating

**Files:**
- Modify: `web/default/src/features/quota-forecast/quota-forecast-display.tsx`
- Modify: `web/default/src/features/dashboard/components/overview/summary-cards.tsx`

- [ ] **Step 1: Add the stats display variant and remove dashboard truncation**

Extend the variant type:

```ts
variant?: 'table' | 'dashboard' | 'stats'
```

For `stats`, render the exact formatted exhaustion timestamp when predicted and the existing status label otherwise. For `dashboard`, keep the duration as the first line and render the exact timestamp with `whitespace-nowrap`, `tracking-tight`, and a compact responsive font instead of `truncate`.

- [ ] **Step 2: Reserve enough dashboard width**

Change the right overview column from `19rem` to `21rem`. Change its metric grid to a narrow recent-usage column and a forecast column with a `10.5rem` minimum width:

```tsx
<div className='grid grid-cols-[minmax(0,1fr)_minmax(10.5rem,1.35fr)] gap-2'>
```

- [ ] **Step 3: Run targeted lint and typecheck**

Run:

```bash
bunx oxlint src/features/quota-forecast/quota-forecast-display.tsx src/features/dashboard/components/overview/summary-cards.tsx
bun run typecheck
```

Expected: no lint errors and typecheck exit code 0.

### Task 3: Add the forecast to common usage logs

**Files:**
- Modify: `web/default/src/features/usage-logs/components/common-logs-stats.tsx`

- [ ] **Step 1: Allow stats badge values to use shared components**

Change `StatBadge.value` from `string | number` to `ReactNode` and keep the current typography wrapper so existing Usage/RPM/TPM badges remain unchanged.

- [ ] **Step 2: Fetch the cached self forecast independently**

Add a second `useQuery` using `SELF_QUOTA_FORECAST_QUERY_KEY`, `getSelfQuotaForecast`, `QUOTA_FORECAST_STALE_TIME`, `retry: 1`, and `enabled: !isAdmin`. This query must not be awaited by the log stats query.

- [ ] **Step 3: Render a page-native forecast badge**

After the Usage badge, render a `Runway` badge only when `!isAdmin`. Use the shared `QuotaForecastDisplay` with `variant='stats'`, pass its loading/error states, and select the accent bar from `getQuotaForecastTone`:

```tsx
<StatBadge
  label={t('Runway')}
  value={
    <QuotaForecastDisplay
      forecast={forecast}
      isLoading={forecastQuery.isLoading}
      isError={forecastQuery.isError || forecastQuery.data?.success === false}
      variant='stats'
    />
  }
  accent={forecastAccent}
/>
```

- [ ] **Step 4: Run targeted lint and typecheck**

Run:

```bash
bunx oxlint src/features/usage-logs/components/common-logs-stats.tsx src/features/quota-forecast/api.ts src/features/quota-forecast/quota-forecast-display.tsx src/features/dashboard/components/overview/summary-cards.tsx
bun run typecheck
```

Expected: no lint errors and typecheck exit code 0.

### Task 4: Verify and commit the implementation

**Files:**
- Test: `web/default/src/features/quota-forecast/lib.test.ts`
- Verify: all modified frontend files

- [ ] **Step 1: Run quota forecast tests**

Run: `bun test src/features/quota-forecast/lib.test.ts`

Expected: all tests pass.

- [ ] **Step 2: Run production build**

Run: `bun run build`

Expected: Rsbuild completes successfully.

- [ ] **Step 3: Check the diff**

Run:

```bash
git diff --check
git diff -- web/default/src/features/quota-forecast web/default/src/features/dashboard/components/overview/summary-cards.tsx web/default/src/features/usage-logs/components/common-logs-stats.tsx
```

Expected: no whitespace errors and only the planned display/query changes.

- [ ] **Step 4: Commit**

```bash
git add web/default/src/features/quota-forecast/api.ts web/default/src/features/quota-forecast/quota-forecast-display.tsx web/default/src/features/dashboard/components/overview/summary-cards.tsx web/default/src/features/usage-logs/components/common-logs-stats.tsx
git commit -m "fix: refine quota forecast displays"
```

### Task 5: Deploy work then master

**Files:**
- Remote work compose: discover from `friday-new-api-worker` labels
- Remote master compose: `/data/frimodel/master/docker-compose.yml`

- [ ] **Step 1: Build one complete workspace image**

Build a `linux/amd64` tar from the full current workspace so all previously deployed uncommitted functionality remains included:

```bash
docker buildx build --builder xmodel-manual-builder --platform linux/amd64 -t frimodel/new-api:quota-forecast-display-20260730 --output type=docker,dest=/tmp/new-api-quota-forecast-display-20260730-amd64.tar .
```

- [ ] **Step 2: Upload and verify the checksum**

Upload through `ubuntu@148.113.178.75`, compare local and remote SHA256 values, and abort before any switch if they differ.

- [ ] **Step 3: Back up and recreate work**

Resolve the worker compose path and service name from Docker labels. Tag the current worker image with a timestamped backup, create an explicit restore script, load and tag the new image, then recreate only the worker service. Wait for healthy status and restart count zero.

- [ ] **Step 4: Back up and recreate master**

Tag the current master image with a timestamped backup, create an explicit restore script, point `frimodel/new-api:latest` to the loaded image, and recreate only `new-api-master` from `/data/frimodel/master/docker-compose.yml`. Wait for healthy status and restart count zero.

- [ ] **Step 5: Verify production**

Verify:

```text
https://platform.frimodel.com/api/status -> 200
https://api.frimodel.com/api/status -> 200
/api/user/self/quota-forecast without auth -> 401
/usage-logs/common -> 200
```

Confirm the worker and master image IDs match the loaded image, CPA and Nginx container IDs remain unchanged, and recent worker/master logs contain no panic, fatal, migration failure, database error, or startup failure.

- [ ] **Step 6: Remove only temporary tar files**

Delete the explicit local and remote `/tmp/new-api-quota-forecast-display-20260730-amd64.tar` files after all verification succeeds. Keep both backup image tags and restore scripts.

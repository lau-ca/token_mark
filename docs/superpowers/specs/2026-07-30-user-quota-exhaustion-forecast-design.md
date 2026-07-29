# User Quota Exhaustion Forecast Design

## Goal

Predict when a user's current quota will be exhausted if the user does not recharge and their recent consumption trend continues.

The prediction must appear:

- In each visible row on the admin `/users` page.
- In the existing balance and runway area on the ordinary user's dashboard overview.

The admin user list must retain its current loading speed. Forecast calculation failure must not prevent the list from loading or being used.

## Scope

This change covers quota-consumption forecasting and its two display surfaces. It does not add automatic recharge, notifications, forecasting based on subscription renewal, or administrative filtering and sorting by forecast.

## Prediction Definition

The forecast assumes:

- No recharge, redemption, transfer, manual quota adjustment, or subscription reset occurs after calculation.
- Future consumption follows the weighted average of the most recent seven rolling 24-hour windows.
- Only consumed quota contributes to the usage rate. Refund and management logs do not count as consumption.

The seven-day window is also the inactivity boundary. If total consumption in the most recent seven days is zero, do not look further back and do not extrapolate from older activity. Return `no_recent_usage` without an exhaustion timestamp. This prevents a user who has been inactive for a long time from receiving a misleading forecast based on stale behavior.

Let `U1` be consumption during the most recent 24 hours, `U2` the preceding 24 hours, and so on through `U7`. Use weights `7, 6, 5, 4, 3, 2, 1` respectively.

```text
weighted_daily_usage = sum(Ui * Wi) / sum(Wi)
remaining_seconds = current_quota / weighted_daily_usage * 86400
predicted_exhausted_at = calculated_at + remaining_seconds
```

For accounts with less than seven days of history, only windows after account creation participate in the denominator. A partial first window is prorated by its available duration. This prevents new accounts from being treated as if their missing history were seven days of zero consumption.

The API returns one of these states:

- `predicted`: current quota and sufficient consumption history produce a forecast.
- `depleted`: current quota is zero or negative.
- `sampling`: the account has less than 24 hours of usable history.
- `no_recent_usage`: usable history exists but total consumption during the seven-day window is zero; no forecast is calculated.
- `unavailable`: the forecast could not be calculated safely.

Predictions beyond 365 days remain valid in the API, while the UI presents them as "more than one year" to avoid false precision.

## Data Source

Use `quota_data`, not raw `logs`.

`quota_data` is already the project's hourly consumption aggregate and is used by the dashboard. Querying it keeps the forecast consistent with existing usage statistics and avoids repeatedly scanning high-volume request logs.

The aggregate can lag by the configured data-export interval. This is acceptable for an approximate forecast and must be explained in the forecast tooltip as a recent-usage estimate rather than an exact billing deadline.

Add a cross-database GORM composite index on:

```text
(user_id, created_at)
```

The migration must remain compatible with SQLite, MySQL, and PostgreSQL.

## Backend Architecture

### Forecast service

Add a focused service that:

1. Accepts a bounded set of user IDs.
2. Loads current quota and account creation time from the primary user database.
3. Reads seven days of hourly `quota_data` for those users with one grouped query.
4. Buckets the hourly totals into seven rolling 24-hour windows in Go.
5. Stops forecasting when all seven windows contain zero consumption instead of consulting older usage.
6. Calculates a result for every requested user, including users with no matching usage rows.

The database query should group by `user_id` and `created_at`. Time-window weighting stays in Go to avoid database-specific date functions.

### Endpoints

Add:

```text
POST /api/user/quota-forecast
GET  /api/user/self/quota-forecast
```

The admin request body contains the IDs for the currently visible page. Reject empty requests and requests containing more than 100 unique positive IDs. The backend loads quota values itself and never trusts balances supplied by the client.

The self endpoint obtains the authenticated user ID from context and uses the same service and response type.

Example admin response item:

```json
{
  "user_id": 123,
  "status": "predicted",
  "calculated_at": 1785340800,
  "predicted_exhausted_at": 1785623400,
  "weighted_daily_usage": 185000,
  "sample_hours": 168
}
```

### Cache

Cache each user's forecast for five minutes. A process-local bounded TTL cache is sufficient because the forecast is approximate and must not introduce a Redis dependency into page loading.

Cache entries are keyed by user ID. The service must compare the cached quota snapshot with the user's current quota; a recharge or administrator quota adjustment therefore invalidates the cached result logically even before TTL expiry.

Do not modify the existing `/api/user/` or `/api/user/search` response path to calculate forecasts.

## Admin `/users` Data Flow

1. The existing users query loads and renders exactly as it does now.
2. After the current page's user IDs are available, React Query requests forecasts for those IDs.
3. Forecast data uses a separate query key and `staleTime` matching the five-minute backend TTL.
4. Previous forecast data may remain visible during pagination refresh, but it must be mapped strictly by user ID so values cannot appear on another user's row.
5. Forecast request errors remain local to forecast cells. They do not trigger a page-level error toast and do not replace user rows.

The request is limited to the current page. Search, filtering, and pagination therefore do not calculate forecasts for off-screen users.

## Admin `/users` Presentation

Keep the forecast inside the existing quota column, below the quota progress bar. Do not add another table column.

Normal display:

```text
12.40 / 50.00
----------------
Expected Aug 3, 14:30
```

Cell states:

- Loading: a short inline skeleton below the progress bar.
- `predicted`: localized forecast time.
- `depleted`: "Depleted" with destructive styling.
- `sampling`: "Collecting usage data".
- `no_recent_usage`: "No recent usage, no forecast".
- Request failure or `unavailable`: "Forecast unavailable".

Color treatment:

- Less than or equal to 24 hours remaining: destructive.
- More than 24 hours and less than or equal to three days: warning.
- More than three days: muted text.

A tooltip shows the exact timestamp, approximate remaining duration, weighted daily usage, seven-day weighted-average basis, and the no-recharge assumption.

Mobile cards use the same quota-cell content and allow the forecast line to wrap instead of increasing the card width.

## Ordinary User Presentation

Reuse the existing dashboard overview balance panel in `SummaryCards`. It already shows current credit, recent usage, and a runway value, so this is the correct semantic location.

Replace the local 24-hour runway calculation with the shared self-forecast API result. Display:

```text
Runway
About 3 days 5 hours
Expected Aug 3, 14:30
```

The current quota, recent 24-hour consumption, and sparklines continue to load through their existing paths. Forecast failure affects only the runway block.

Do not place the forecast in usage-log statistics. Log statistics respond to model, token, group, and time filters, while account exhaustion is an account-level forecast that must not appear to change with those filters.

## Internationalization

All new UI text uses `useTranslation()` and is added to every supported locale through the project's i18n synchronization workflow.

Time formatting uses the existing frontend date utilities and the browser's current locale/timezone. The API returns Unix timestamps and does not format user-facing dates.

## Error Handling

- An admin forecast request may partially succeed. The response includes a result for every requested valid user ID.
- Database or calculation errors produce `unavailable`; they do not block the user list or dashboard.
- Non-finite, zero, or negative calculated rates never produce a timestamp.
- Timestamp arithmetic must guard against overflow before conversion.
- Duplicate user IDs are deduplicated before querying and counting against the limit.

## Testing

Backend tests protect:

- Weight order and normalization for seven full windows.
- New-account normalization and partial first-window prorating.
- `depleted`, `sampling`, and `no_recent_usage` states.
- Seven days of zero consumption never falls back to activity older than the forecast window.
- Forecast timestamp calculation and overflow protection.
- Admin authorization, self-user scoping, ID deduplication, and the 100-user limit.
- One grouped data query serving multiple users.
- Cache reuse and quota-snapshot invalidation.
- SQLite behavior for the composite index and grouped query; implementation remains dialect-neutral for MySQL and PostgreSQL.

Frontend tests protect:

- Forecasts map to rows by user ID across pagination changes.
- The user table remains usable while forecasts load or fail.
- Threshold styling and special-state labels.
- The dashboard uses the self-forecast result and degrades locally on failure.

Validation includes targeted Go tests, frontend type checking, linting changed frontend files, i18n synchronization, and a production frontend build. End-to-end browser testing is outside this task unless explicitly requested.

## Rollout and Observability

Record forecast query duration, requested user count, cache hit count, and calculation failures in structured backend logs without logging balances or usernames.

After deployment, verify:

- `/users` list response latency is unchanged because its endpoint was not modified.
- Forecast requests contain only visible user IDs.
- Forecast query latency remains bounded with 20 and 100 users.
- Ordinary users only receive their own forecast.
- Recharge or quota adjustment changes the next forecast despite an existing cache entry.

If forecast aggregation becomes expensive at substantially larger scale, the service boundary allows replacement with scheduled precomputation without changing either frontend surface.

# Channel Daily Health Status Design

## Goal

Display a channel health status based on the current Beijing-day error rate, refresh it through the same manual, bulk, and scheduled mechanisms as channel balance, and improve the channel list presentation of usage, balance, and health data.

## Time Range

- The reporting timezone is fixed to `Asia/Shanghai`.
- Each refresh covers the current Beijing day from 00:00:00 through the refresh time.
- A stored health snapshot from a previous Beijing calendar date is shown as pending refresh rather than reused as current status.

## Health Calculation

Use `model.LOG_DB`, because deployments may store logs separately from the main database.

For the selected channel and time range:

- Successful attempts are logs with `type = LogTypeConsume`.
- Failed attempts are logs with `type = LogTypeError`.
- Exclude rows with `channel_id = 0`.
- Total attempts are successes plus failures.
- Error rate is `failures / total attempts * 100`.

The existing relay path records failed retry attempts against the channel that failed and records successful consumption against the channel that completed the request. This makes the attempt-level ratio suitable for channel health.

If consume-log recording is disabled, the health status is unknown because the success denominator is incomplete.

## Status Rules

- `unknown`: fewer than 20 attempts, consume logging disabled, or no current-day snapshot.
- `healthy`: error rate below 2%.
- `warning`: error rate from 2% up to but not including 10%.
- `critical`: error rate at or above 10%.

Persist the current snapshot on the channel:

- status
- error rate
- success count
- error count
- total count
- Beijing date represented by the snapshot
- updated timestamp

Persisting the snapshot keeps channel-list reads cheap and lets all existing refresh triggers share the same result.

## Refresh Behavior

Keep the existing triggers:

- Clicking the balance value for one channel.
- Refreshing all enabled channel balances.
- Scheduled channel balance refresh.

Each trigger refreshes balance and health independently:

- A balance-query failure does not prevent health statistics from updating.
- A health-query failure does not overwrite the stored balance.
- Manual refresh returns enough information for the frontend to refresh the row even if one half fails.
- Bulk and scheduled refresh continue processing other channels and log failures without exposing credentials.

Bulk and scheduled refresh aggregate current-day health statistics for all relevant channels in one grouped `LOG_DB` query rather than running one log query per channel.

## Channel List Layout

Place the new `Status (error rate)` column immediately after `Usage / Balance`.

The two data columns use consistent layout rules:

- All labels and values are left-aligned.
- Each row uses the same height and vertical gap.
- Labels use a fixed width so values begin at the same horizontal position.
- Monetary values and percentages use tabular numerals.
- The usage/balance column receives enough fixed width for two stacked rows.
- The health column receives enough fixed width for status plus percentage without wrapping.
- Long details remain in tooltips rather than widening the table.

Usage/balance cell:

```text
Used      $123.50
Balance   $51.60
```

The balance value remains clickable and keeps its existing refresh affordance.

Health cell:

```text
Status    Healthy
Error     0.8%
```

The tooltip includes success count, error count, total attempts, Beijing reporting date, and last update time. Unknown and pending-refresh states display compact neutral text.

The mobile channel card uses the same labels, order, colors, and left-aligned spacing.

## Data and Database Compatibility

- Add health snapshot fields to the existing `channels` table through GORM migration.
- Support SQLite, MySQL, and PostgreSQL.
- Use GORM conditional aggregation compatible with all three databases.
- Explicitly persist zero counts and zero error rates; do not rely on struct updates that skip zero values.
- Existing channels start with an unknown health status until refreshed.

## Security and Privacy

- The health API returns aggregate counts only.
- Do not expose log content, user IDs, token IDs, request IDs, or credentials in channel-list responses.
- Health-query error logs include only channel identity and a sanitized reason.

## Testing

Backend tests cover:

- Beijing-day start and end boundaries.
- Success/error aggregation by channel.
- Exclusion of other channels, other log types, and `channel_id = 0`.
- Unknown, healthy, warning, and critical thresholds.
- Exact behavior at 2% and 10%.
- Explicit persistence of zero-valued snapshots.
- Independent balance and health refresh failures.
- Batch aggregation for multiple channels.

Frontend tests cover:

- Current-day health state selection.
- Previous-day snapshots becoming pending refresh.
- Status label, percentage, and color mapping.
- Stable usage/balance and health cell formatting.

Verification includes targeted and full backend tests, frontend unit tests, type checking, linting of changed files, production build, and i18n synchronization. Browser end-to-end testing is excluded unless explicitly requested.

## Deployment

After verification, build a clean `linux/amd64` image from committed source and manually deploy only `new-api-master` on `157.230.213.186`.

- Do not use deployment scripts.
- Back up the current master image and compose configuration first.
- Upload and checksum the image archive manually.
- Recreate only `new-api-master`.
- Verify database migration, container health, localhost endpoints, public endpoints, and that worker, CPA, and nginx were not recreated.

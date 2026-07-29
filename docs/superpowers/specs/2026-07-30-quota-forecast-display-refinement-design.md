# Quota Forecast Display Refinement

## Goal

Keep the complete predicted exhaustion timestamp visible on the dashboard and add the same information to `/usage-logs/common` without disrupting either page's established visual hierarchy or loading path.

## Dashboard

- Keep the existing two-metric layout for recent usage and runway.
- Give the runway metric more horizontal space than the recent-usage metric.
- Render the predicted exhaustion timestamp on one line without truncation.
- Preserve the existing duration, tone, loading state, and explanatory tooltip.
- Use responsive text sizing rather than allowing the timestamp to overflow its card.

## Common Usage Logs

- Add a compact forecast badge beside the existing Usage, RPM, and TPM badges.
- Match the existing badge height, border, background, typography, spacing, and accent-bar treatment.
- Show the exact exhaustion timestamp as the badge value when a prediction exists.
- Reuse the shared forecast status and tooltip behavior for depleted, sampling, inactive, and unavailable states.
- Show the forecast only when the page is scoped to the signed-in user's own logs. Hide it in the admin "All" scope because a single account forecast would be misleading there.

## Data Flow and Performance

- Reuse `GET /api/user/self/quota-forecast`.
- Fetch it independently from the log list and log-stat requests so neither page load path waits on the forecast.
- Use the existing five-minute React Query stale time and a stable shared query key so dashboard and usage logs reuse cached data during navigation.
- Do not add backend queries or change the log-list API.

## Verification

- Unit-test the display formatting and non-truncating variant contract where practical.
- Run targeted frontend tests, typecheck, lint, and production build.
- Deploy work first and master last, with separate image backups and restore scripts.
- Verify both services are healthy with restart count zero, public status endpoints return 200, forecast routes remain authenticated, and unrelated CPA/Nginx containers remain unchanged.

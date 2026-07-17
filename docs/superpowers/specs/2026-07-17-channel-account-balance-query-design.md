# Channel Account Balance Query Design

## Goal

Replace the existing channel API-key remaining-quota calculation with account-level balance queries for upstream New API and Sub2API platforms. Keep the existing manual, bulk, and scheduled refresh triggers unchanged.

## Scope

- Support single-key channels only. Multi-key channels remain unsupported by balance refresh.
- Require an explicit balance platform and balance query base URL.
- Query New API account wallet balance with account credentials.
- Query Sub2API account wallet balance with the channel's existing API key.
- Store the normalized monetary result in the existing `channels.balance` USD field.
- Remove the legacy OpenAI-compatible subscription-minus-usage balance calculation without fallback.

## Channel Configuration

Add dedicated channel fields:

- `balance_platform`: empty, `new_api`, or `sub2api`.
- `balance_base_url`: upstream platform base URL used only for balance queries.
- `balance_user_id`: New API account user ID; unused for Sub2API.
- `balance_auth_key`: New API account access token; unused for Sub2API.

The frontend channel editor shows:

- Balance platform type.
- Balance query URL.
- New API user ID when `balance_platform = new_api`.
- New API account access token when `balance_platform = new_api`.

Sub2API does not need another credential field. It reuses the channel's existing API key.

The balance query URL is a base URL such as `https://example.com`. The backend removes trailing slashes and appends fixed platform paths.

## Credential Security

`balance_auth_key` is a sensitive channel field.

- Channel list and channel detail responses must not expose its value.
- Responses expose only whether an account access token is configured.
- On edit, an empty account access-token input preserves the stored value.
- Replacing the token requires channel sensitive-write permission.
- Audit records may state that the credential changed but must never contain the credential value.
- The credential remains available only to backend balance-query code.

## New API Balance Query

Required configuration:

- `balance_base_url`
- positive `balance_user_id`
- non-empty `balance_auth_key`

Requests:

1. `GET {balance_base_url}/api/status`
2. `GET {balance_base_url}/api/user/self`

The account request sends:

```http
Authorization: Bearer <balance_auth_key>
New-Api-User: <balance_user_id>
Accept: application/json
```

The status response must provide a positive numeric `data.quota_per_unit`. The account response must have `success = true`, the expected user ID, and a non-negative numeric `data.quota`.

The normalized balance is:

```text
balance_usd = data.quota / data.quota_per_unit
```

The backend must reject invalid, missing, non-finite, or negative values instead of storing them.

## Sub2API Balance Query

Required configuration:

- `balance_base_url`
- non-empty existing channel API key

Request:

```http
GET {balance_base_url}/v1/usage
Authorization: Bearer <channel key>
Accept: application/json
```

The response is accepted only when all of the following hold:

- `balance` is present, numeric, finite, and non-negative.
- `unit` equals `USD`, case-insensitively.
- The response represents wallet balance rather than key quota or subscription quota.

The backend must not use `remaining`, `quota.remaining`, subscription limits, or usage totals as account balance. A quota-limited or subscription-only response returns an explicit unsupported-balance error.

## Refresh Behavior

The existing triggers remain unchanged:

- Refresh one channel.
- Refresh all enabled channels.
- Scheduled refresh of enabled channels.

Per-channel behavior:

1. Reject multi-key channels.
2. Reject missing or unsupported `balance_platform`.
3. Validate all platform-specific configuration.
4. Query the selected upstream platform.
5. Update `balance` and `balance_updated_time` only after a fully valid response.

Manual refresh returns the validation or upstream error to the caller. Bulk and scheduled refresh continue processing other channels as they do today, while logging the channel ID, channel name, and failure reason without credentials.

The old `/v1/dashboard/billing/subscription` and `/v1/dashboard/billing/usage` difference calculation is removed. There is no compatibility fallback when account-balance configuration is absent.

## Data Compatibility

- Add the new columns through the existing GORM migration path so SQLite, MySQL, and PostgreSQL remain supported.
- Existing channels receive empty balance configuration and fail balance refresh with a clear configuration error until updated.
- The existing `balance` column and frontend balance display remain unchanged.
- Copying or batch-creating channels copies the non-secret balance settings consistently with existing channel fields. Secret handling follows the same preservation rules as channel credentials.

## Error Handling

Errors must distinguish at least:

- Balance platform not configured.
- Balance query URL not configured or invalid.
- New API user ID not configured.
- New API account access token not configured.
- Sub2API channel API key not configured.
- Unsupported multi-key channel.
- Upstream authentication failure.
- Unexpected upstream HTTP status.
- Invalid upstream response.
- Sub2API response contains only key or subscription quota, not wallet balance.

Errors returned to administrators must not include access tokens, API keys, or authorization headers.

## Testing

Backend tests cover observable contracts:

- New API converts raw quota using the upstream `quota_per_unit`.
- New API sends the account access token and matching user ID headers.
- New API rejects mismatched user IDs and malformed responses.
- Sub2API accepts a wallet response containing `balance` and `unit = USD`.
- Sub2API rejects quota-limited and subscription-only responses.
- Missing platform-specific fields produce clear errors and do not update stored balance.
- Non-200 responses, invalid JSON, negative values, NaN, and infinity do not update balance.
- Multi-key channels remain unsupported.
- Sensitive account credentials are omitted from channel list/detail responses and protected by sensitive-write authorization.

Frontend tests cover:

- Platform-dependent fields appear correctly.
- Balance configuration fields round-trip through create and edit payloads.
- Sub2API does not request duplicate credentials.
- Editing with an empty account access-token field preserves the existing secret.

Verification includes targeted Go tests, frontend type checking, linting of changed files, i18n synchronization, and the frontend production build. Browser end-to-end testing is outside this task unless explicitly requested.

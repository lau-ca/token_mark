# Model Success Rate API Design

## Goal

Provide an authenticated read-only API that accepts the model identifier shown on `/pricing` and returns the model's request success and error rates for the current Beijing calendar day and the rolling last five minutes.

## Existing Behavior

`GET /api/perf-metrics` already exposes model performance data, but it does not satisfy this contract:

- it accepts a model name but returns group series rather than the two requested windows;
- access depends on the pricing navigation setting and can be public;
- the default hourly performance bucket cannot produce an exact rolling five-minute rate;
- aggregated performance metrics do not retain HTTP status classes, so they cannot separate client 4xx errors from server or upstream 5xx errors.

## API Contract

### Request

```http
GET /api/model/success-rate?model=gpt-image-2-w
Authorization: Bearer <profile access token>
New-Api-User: <user id>
```

The route uses `middleware.UserAuth()`, matching the account access token generated on `/profile`. The `model` query parameter is the exact API model identifier displayed on `/pricing`, not the numeric primary key from the `models` table.

### Response

```json
{
  "success": true,
  "data": {
    "model": "gpt-image-2-w",
    "timezone": "Asia/Shanghai",
    "generated_at": 1785210000,
    "today": {
      "start_timestamp": 1785168000,
      "end_timestamp": 1785210000,
      "request_count": 110925,
      "success_count": 97744,
      "client_error_count": 5545,
      "server_error_count": 7636,
      "other_error_count": 0,
      "success_rate_including_4xx": 88.1172,
      "success_rate_excluding_4xx": 92.7538,
      "error_rate_including_4xx": 11.8828,
      "error_rate_excluding_4xx": 7.2462
    },
    "last_5_minutes": {
      "start_timestamp": 1785209700,
      "end_timestamp": 1785210000,
      "request_count": 3292,
      "success_count": 3002,
      "client_error_count": 221,
      "server_error_count": 69,
      "other_error_count": 0,
      "success_rate_including_4xx": 91.1908,
      "success_rate_excluding_4xx": 97.7532,
      "error_rate_including_4xx": 8.8092,
      "error_rate_excluding_4xx": 2.2468
    }
  }
}
```

Rates are percentages rounded to four decimal places. When a denominator is zero, the corresponding rate is `null`; counts remain zero.

## Statistical Semantics

The logs table is the authoritative source because it contains exact request timestamps and error status codes.

- A request is identified by `request_id` and counted once.
- A consume log (`type = 2`) makes the request successful.
- A request with error logs (`type = 5`) and no consume log in the same window is failed.
- When a failed request has multiple error logs, only its latest error determines the status class.
- HTTP 400 through 499 are client errors.
- HTTP 500 through 599 are server or upstream errors.
- Missing or nonstandard status codes are other errors and are included in the rate that excludes 4xx.
- `error_rate_including_4xx = all failed requests / all requests`.
- `error_rate_excluding_4xx = (server errors + other errors) / (successful requests + server errors + other errors)`.
- Success rates are the complements under their corresponding denominator.

The day window begins at midnight in `Asia/Shanghai`. The instant window is `[now - 5 minutes, now]`.

## Architecture

### Router

Register `GET /api/model/success-rate` in `router/api-router.go` with `middleware.UserAuth()`.

### Controller

Add `controller/model_success_rate.go` to:

- require a non-empty model query parameter;
- call the statistics service;
- return the standard `{success, data}` response;
- return a database error without exposing SQL or log contents.

### Service

Add `service/model_success_rate.go` to:

- calculate the Beijing day and rolling five-minute ranges;
- request both window aggregates from the model layer;
- calculate counts and nullable rates;
- cache the completed response briefly by exact model identifier.

The cache lifetime is 15 seconds. Redis is used when available so repeated callers and multiple application instances share the result; an in-process cache is not required.

### Model

Add `model/model_success_rate.go` with GORM/raw aggregate queries against `LOG_DB`.

The query must support SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+. Common request counting and final-error selection use portable SQL. Extracting `other.status_code` uses explicit database-type branches:

- PostgreSQL JSON text extraction;
- MySQL `JSON_EXTRACT` with text conversion;
- SQLite `json_extract`.

Every branch returns the same aggregate fields and parameterizes model identifiers and timestamps.

## Validation and Failure Handling

- Empty or whitespace-only model identifiers return HTTP 400.
- Model identifiers are exact-match parameters; wildcard syntax is not accepted.
- Authentication failures follow the existing profile access-token response behavior.
- Database failures return HTTP 500 and are logged internally.
- No-data windows return zero counts and `null` rates.
- The response never includes error messages, prompts, user identifiers, channel identifiers, or raw log metadata.

## Testing

Add deterministic backend tests covering:

- authentication is required on the new route;
- missing model returns HTTP 400;
- successful requests and final failures are deduplicated by `request_id`;
- a consume log overrides earlier error logs for the same request;
- the latest error controls classification when a failed request has multiple errors;
- 4xx and 5xx counts produce both requested rate denominators;
- zero-request windows return `null` rates;
- exact model matching prevents another model's logs from entering the result;
- Beijing day and rolling five-minute boundaries are respected.

Tests use `testify/require` for setup and fatal checks and `testify/assert` for value checks.

## Deployment Boundary

The new endpoint is an `/api` management-plane route served by master. Only the master image needs deployment. Worker deployment is not required and must not be performed without separate user authorization.

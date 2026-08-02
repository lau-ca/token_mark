# Task Seconds/Duration Billing Normalization Design

## Goal

Ensure request-aware task billing receives the same video duration whether a client sends `duration` or the compatible `seconds` field. Existing `duration` behavior remains unchanged, while conflicting values are rejected.

## Current Behavior

`TaskSubmitReq` preserves `duration` as an integer and `seconds` as a string. Generic task validation bounds either value, but tiered task billing builds its normalized expression input only from `TaskSubmitReq.Duration`. A request such as `{"seconds":"5"}` therefore reaches `param("duration")` as zero and cannot be charged accurately per second.

## Considered Approaches

1. Mutate every stored task request so `Duration` is populated from `Seconds`. This centralizes the value but changes the request object observed by every adaptor.
2. Resolve the duration only inside tiered billing. This is narrow, but duplicates parsing and leaves validation and billing with separate rules.
3. Add a shared task-duration resolver used by validation and tiered billing. This keeps the original request intact while giving both boundaries one consistent interpretation.

Approach 3 is selected because it is reusable, minimizes downstream behavior changes, and prevents validation and billing from drifting apart.

## Design

Add a shared resolver in `relay/common` with these rules:

- If `duration` is present, use it.
- If only `seconds` is present, parse and use it.
- If both are present, require equal integer values.
- If neither is present, return zero so existing provider defaults remain possible.
- Return an error for malformed or conflicting values.

Use the resolver in generic task-duration validation before applying the existing `MaxTaskDurationSeconds` bound. Use the same resolver in tiered task billing when constructing the normalized expression request body.

The billing expression for one dollar per second remains:

```text
tier("per_second", per_request(1) * param("duration"))
```

## Compatibility

- Existing `duration` requests keep the same value and billing behavior.
- Existing fixed-price task models remain unchanged.
- Valid `seconds` requests gain correct duration-aware billing.
- Requests containing conflicting `duration` and `seconds` values are rejected before charging or relay.
- No database migration or API response change is required.

## Tests

Add deterministic regression coverage for:

- `duration`-only resolution.
- `seconds`-only resolution.
- matching `duration` and `seconds`.
- conflicting values.
- tiered task billing charging a `seconds` request by its normalized duration.
- existing `duration`-based tiered billing remaining unchanged.

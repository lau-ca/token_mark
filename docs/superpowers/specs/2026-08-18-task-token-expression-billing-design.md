# Task Token Expression Billing Design

## Goal

Allow asynchronous video tasks to use a billing expression that selects a per-million-token price from normalized request fields and settles against the upstream task's actual `usage.completion_tokens`.

The initial use case is `Doubao-Seedance-2.0`:

| Resolution | No reference video | Has reference video |
|---|---:|---:|
| 480p / 720p | 46 | 28 |
| 1080p | 51 | 31 |
| 4K | 26 | 16 |

## Scope

- Add an explicit opt-in path for token-based Task expressions.
- Expose normalized `resolution`, `duration`, and `has_reference_video` fields to Task expressions.
- Freeze the expression and normalized request values when the task is submitted.
- Re-run the frozen expression with the upstream `completion_tokens` after successful completion.
- Settle the difference through the existing task quota adjustment path.
- Preserve all existing per-request Task expressions and legacy ratio billing.
- Configure `Doubao-Seedance-2.0` through the model-pricing page after deployment.

This change does not alter database tables, group ratios, other providers, or models that do not explicitly select token-based Task expression billing.

## Expression Contract

Token-based Task expressions use an explicit marker so existing Task expressions cannot silently change behavior. The expression receives:

- `c`: actual upstream completion tokens during final settlement.
- `param("resolution")`: normalized output resolution.
- `param("duration")`: normalized requested output duration.
- `param("has_reference_video")`: whether the normalized request contains a reference video.

The Seedance expression will select the official price and multiply it by `c`:

```text
task_tokens(
  param("resolution") == "4k"
    ? tier("4k", c * (param("has_reference_video") ? 16 : 26))
    : param("resolution") == "1080p"
      ? tier("1080p", c * (param("has_reference_video") ? 31 : 51))
      : tier("480p_720p", c * (param("has_reference_video") ? 28 : 46))
)
```

Expression coefficients remain prices per one million tokens. Existing expression quota conversion therefore produces:

```text
completion_tokens / 1,000,000 * selected_price * group_ratio
```

## Request Normalization

The Task billing helper will derive `has_reference_video` from the normalized request rather than from provider-specific raw JSON. It must recognize the existing supported paths:

- `ReferenceVideos`
- native `content` entries with `type = video_url`
- compatible metadata content containing `video_url`

The normalized values are stored in the task billing context so polling workers do not need the original HTTP request.

## Submission and Pre-consume

At submission time, the system compiles and freezes the expression together with the normalized request fields. It preserves the existing Task pre-consume behavior because the upstream completion-token count is not available yet.

Per-request expressions containing `per_request()` continue to calculate their complete charge at submission and remain final. Legacy ratio-based tasks continue unchanged.

## Completion Settlement

When a task succeeds and returns positive completion tokens:

1. Detect the frozen token-based Task expression snapshot.
2. Rebuild the expression request input from the stored normalized fields.
3. Run the frozen expression with `c = completion_tokens`.
4. Apply the frozen/final group ratio using the existing quota conversion helpers.
5. Use `RecalculateTaskQuota` to apply a supplementary charge or refund.
6. Record the matched tier and any quota-saturation marker in task billing logs.

Failed, cancelled, or expired tasks retain the existing refund behavior and do not receive a successful-token settlement.

## Compatibility and Isolation

- The new behavior requires the explicit `task_tokens()` marker.
- Existing `per_request()` Task expressions are unchanged.
- Expressions without either Task marker retain legacy behavior.
- Non-Task relay expressions are unchanged.
- Models without the new expression are unchanged.
- The Doubao hard-coded price table is not required for `Doubao-Seedance-2.0` after the expression is configured.

## Public Pricing Presentation

The pricing page must parse the supported `task_tokens(...)` expression into the same normalized dynamic-tier data used by model cards, table rows, model details, group pricing, and usage-log details.

For the Seedance expression, the model card and table row show that the model has three resolutions and six prices. The model detail renders a readable matrix with one row per resolution and separate columns for requests without and with a reference video. Unsupported `task_tokens` shapes keep the existing raw-expression fallback instead of guessing at prices.

The parser is expression-based rather than model-name-based. Other models can use the same supported expression shape without adding aliases or hard-coded model identifiers. This presentation layer does not participate in quota calculation and cannot alter billing results.

## Validation and Tests

Add deterministic tests for:

- all six Seedance price combinations;
- reference-video detection through normalized request fields;
- final settlement using actual completion tokens;
- supplementary charge and refund paths;
- failed tasks not receiving successful-token settlement;
- unchanged `per_request()` Task expressions;
- unchanged legacy ratio billing;
- malformed, negative, NaN, infinite, and saturated expression results.
- complete parsing and display metadata for all six Seedance pricing combinations;
- raw-expression fallback for unsupported Task-token expression shapes.

Run the focused Go tests for billing expressions, Task price calculation, Doubao request normalization, task polling, and task settlement.

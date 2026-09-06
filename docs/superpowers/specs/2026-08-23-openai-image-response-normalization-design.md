# OpenAI Image Response Normalization Design

## Goal

Add an opt-in per-channel compatibility switch that normalizes non-streaming OpenAI-compatible image responses to the current OpenAI image response envelope. Channels with the switch disabled must keep their current response behavior exactly. The normalizer must never modify the upstream `data` value.

The change applies to image generation and image editing responses that use the shared OpenAI-compatible JSON response handler. Streaming SSE responses are outside the scope.

## Selected Approach

Extend the existing channel `settings` object with one boolean flag and apply a targeted JSON patch in the shared OpenAI image non-streaming handler.

The response body remains raw JSON while fields are inspected and patched. This avoids decoding and re-encoding large `data[].b64_json` values, preserves unknown provider fields, and guarantees that the `data` subtree is not rebuilt by a fixed DTO.

The processing order is:

1. Read and validate the upstream response using the existing path.
2. Apply the existing image URL filtering when configured.
3. Apply the existing response parameter overrides.
4. If the new normalization switch is enabled, normalize the top-level response fields and every supported `usage` leaf.
5. Write the resulting JSON response to the client.

The normalizer runs after existing response overrides so an explicit channel override is treated like an upstream value and remains authoritative.

## Alternatives Considered

### Decode the whole response into a fixed image DTO

This would be straightforward, but it could discard unknown top-level fields, change provider-specific `data` members, and copy multi-megabyte Base64 strings. It does not satisfy the requirement that `data` remain untouched.

### Add a normalization layer to every image adaptor

This could cover provider-specific adaptors individually, but it would duplicate the same contract across many channels and increase the risk of inconsistent behavior. The shared OpenAI-compatible response path is the smallest stable boundary for this requirement.

### Patch the shared OpenAI-compatible non-streaming response

This is the selected approach. It centralizes the opt-in behavior, covers generation and editing, and leaves every disabled channel and every streaming response unchanged.

## Channel Setting

Add a boolean field to `dto.ChannelOtherSettings` and the default channel editor. The stored key will describe the complete behavior rather than a single field, for example:

```json
{
  "normalize_openai_image_response": true
}
```

Rules:

- Default is `false`.
- The frontend switch is shown for the same OpenAI-compatible channel types that expose the existing OpenAI image response controls.
- Existing channel JSON is preserved when the form is loaded and saved.
- The setting has no effect on streaming responses.
- The setting has no effect on error responses.
- When disabled, the normalizer is not called and the response follows the current path.

## Normalized Response Contract

The normalizer guarantees these top-level fields while preserving all other upstream top-level fields:

```json
{
  "created": 1787277513,
  "data": [],
  "output_format": "png",
  "quality": "auto",
  "size": "auto",
  "usage": {
    "input_tokens": 0,
    "input_tokens_details": {
      "image_tokens": 0,
      "text_tokens": 0
    },
    "output_tokens": 0,
    "total_tokens": 0,
    "output_tokens_details": {
      "image_tokens": 0,
      "text_tokens": 0
    }
  }
}
```

`data` is not created, replaced, reordered, inspected, or normalized. If the upstream response contains `data`, its raw JSON value is preserved. If the upstream response does not contain `data`, the normalizer does not invent one because doing so would violate the data-preservation boundary.

Unknown top-level provider fields remain present. Only the normalized top-level fields and the `usage` object are patched.

## Top-Level Field Resolution

Each field is resolved independently. A valid upstream value has highest priority except for `quality`, which is derived from the original client request.

### `created`

1. Preserve a valid upstream numeric `created` value, including an explicit zero.
2. If absent or invalid, use the current Unix timestamp in seconds.

### `output_format`

1. Preserve a non-empty upstream string.
2. Otherwise use the normalized original client request value.
3. If the client omitted it or supplied an unusable value, use `png`.

OpenAI's current image API specification defines `png` as the request default.

### `quality`

1. If the normalized original client request value is `low`, `medium`, or `high`, preserve that request value.
2. For every other request value, including an omitted value, `auto`, whitespace-only text, mixed case, or an unknown value, use `medium`.

The upstream response value does not override the client request for this field. This keeps the normalized envelope aligned with the quality requested by the caller while giving unsupported values a stable `medium` fallback.

### `size`

1. Preserve a non-empty upstream string.
2. Otherwise use the normalized original client request value.
3. If the client omitted it or supplied an unusable value, use `auto`.

This field is included because it is part of the standard image response envelope shown by the current OpenAI schema and can be derived without inspecting image data.

## Usage Resolution

The normalized `usage` object always contains every supported leaf, even when the upstream omits the whole object, provides only one branch, uses legacy token names, or returns empty detail objects.

The output shape is fixed to:

```json
{
  "input_tokens": 0,
  "input_tokens_details": {
    "image_tokens": 0,
    "text_tokens": 0
  },
  "output_tokens": 0,
  "total_tokens": 0,
  "output_tokens_details": {
    "image_tokens": 0,
    "text_tokens": 0
  }
}
```

Each leaf is resolved separately:

| Output leaf | Primary upstream path | Fallback upstream path | Missing or invalid |
| --- | --- | --- | --- |
| `input_tokens` | `usage.input_tokens` | `usage.prompt_tokens` | `0` |
| `input_tokens_details.image_tokens` | `usage.input_tokens_details.image_tokens` | `usage.prompt_tokens_details.image_tokens` | `0` |
| `input_tokens_details.text_tokens` | `usage.input_tokens_details.text_tokens` | `usage.prompt_tokens_details.text_tokens` | `0` |
| `output_tokens` | `usage.output_tokens` | `usage.completion_tokens` | `0` |
| `output_tokens_details.image_tokens` | `usage.output_tokens_details.image_tokens` | `usage.completion_tokens_details.image_tokens` | `0` |
| `output_tokens_details.text_tokens` | `usage.output_tokens_details.text_tokens` | `usage.completion_tokens_details.text_tokens` | `0` |
| `total_tokens` | `usage.total_tokens` | none | `0` |

Resolution rules:

- Primary paths win even when the value is explicitly `0`.
- Fallback paths are used only when the primary leaf is absent or invalid.
- The presence of `usage`, a details object, or a sibling leaf never prevents deeper missing leaves from being filled.
- Values must be finite, non-negative JSON numbers representable as Go integers. Negative values, fractional values, strings, booleans, objects, arrays, overflow, and malformed numbers are invalid and become `0` or allow the fallback path to be tried.
- `total_tokens` is not calculated from other fields. If the upstream did not provide a valid value, it remains `0` as requested.
- Provider-specific usage fields such as cache counters, `prompt_tokens`, and `completion_tokens` are not copied into the normalized object.
- The final `usage` object contains only the standard image usage leaves above. This prevents inconsistent mixed schemas while retaining the upstream information that maps to the standard contract.

## Data Preservation

The implementation must not unmarshal `data` into `dto.ImageResponse` or another typed structure.

The helper will read paths with `gjson` and patch only top-level scalar fields plus the complete normalized `usage` object with `sjson`. Tests will compare the raw `data` JSON slice before and after normalization, including provider-specific fields and a representative Base64 string.

The existing image URL hiding feature remains independent. If both switches are enabled, URL hiding runs first because it is an explicitly configured existing transformation. The new normalizer itself never changes `data`.

## Error Handling

- Invalid upstream JSON continues to return the existing bad-response error before normalization.
- A JSON root that is not an object is treated as a bad upstream response when normalization is enabled because standard top-level fields cannot be safely attached.
- A field-level type mismatch does not fail an otherwise successful image response; the invalid field follows its documented fallback or zero rule.
- JSON patch failures use the existing bad-response error path and occur before any response bytes are written.
- Errors and non-2xx responses continue through the existing error handling path and are not normalized.

## Frontend and Internationalization

The default channel editor adds one switch beside the existing OpenAI image response controls. The label and description explain that it affects only non-streaming image generation and editing responses and preserves `data`.

The form schema, defaults, hydration, serialization, sensitive-field allowlist, and channel types are updated consistently. All supported frontend locales receive translations through the existing i18n workflow.

Classic UI is outside this change.

## Testing

Backend regression tests protect these observable contracts:

- Disabled switch returns the original body without normalization.
- Enabled switch preserves the exact raw `data` value.
- Existing upstream top-level values win over request-derived values except for `quality`.
- Missing `output_format` and `size` use request values; `quality` follows its request allowlist rule.
- Missing request values use `png`, `medium`, and `auto` for `output_format`, `quality`, and `size` respectively.
- Missing or invalid `created` uses the current Unix time within a bounded assertion window.
- A complete native image `usage` object is preserved leaf by leaf.
- A legacy `prompt_tokens` and `completion_tokens` usage object maps to the standard image leaves.
- A mixed object resolves every leaf independently and observes primary-path precedence for explicit zero.
- Missing nested details are filled with zero even when parent objects exist.
- Invalid, negative, fractional, and overflowing usage values cannot enter the normalized response.
- Extra upstream usage keys are removed from the normalized `usage` object.
- Generation and editing use the same non-streaming normalization path.
- Streaming SSE chunks and JSON-to-SSE conversion remain unchanged.

Frontend tests protect form hydration and serialization for enabled and disabled channels. Verification includes targeted Go tests, the affected frontend tests, TypeScript checks, lint for changed files, and the frontend i18n synchronization checks. Browser end-to-end testing is outside this task.

## Acceptance Criteria

- Administrators can enable image response normalization per supported channel.
- The setting is disabled by default and disabled channels retain current response behavior.
- Only successful non-streaming JSON image generation and editing responses are normalized.
- `data` is never changed by the normalizer.
- Valid upstream top-level values are authoritative.
- Request-derived and official default values fill only missing top-level response fields.
- Every standard `usage` leaf is present and independently resolved.
- Missing or invalid usage leaves are exactly `0`.
- Streaming responses, error responses, unrelated channel types, and classic UI are unaffected.

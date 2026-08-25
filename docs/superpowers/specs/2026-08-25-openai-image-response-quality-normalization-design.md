# OpenAI Image Response Quality Normalization Design

## Goal

Adjust only the `quality` field produced by the opt-in OpenAI image response normalizer. The normalized response must reflect a valid quality explicitly requested by the client instead of preserving a conflicting upstream value.

All other image response normalization behavior remains unchanged.

## Quality Resolution

Read `quality` from the normalized original client `dto.ImageRequest` stored on `RelayInfo`.

- `low` returns `low`.
- `medium` returns `medium`.
- `high` returns `high`.
- Any other value, including an omitted value, `auto`, whitespace-only text, or an unknown value, returns `medium`.

Matching is exact and case-sensitive, consistent with the accepted OpenAI request values. An upstream top-level `quality` value does not override this result.

## Unchanged Behavior

- The behavior applies only when `normalize_openai_image_response` is enabled.
- Successful non-streaming image generation and editing responses use the same logic.
- `created`, `output_format`, and `size` keep their existing upstream-first resolution.
- `usage` keeps its existing leaf-by-leaf normalization.
- `data` remains byte-for-byte untouched by the normalizer.
- Streaming responses, errors, and channels with the switch disabled remain unchanged.
- Existing response parameter overrides still run before normalization. The normalizer is authoritative only for the final `quality` field when the switch is enabled.

## Implementation

Replace the current generic upstream-first resolution for `quality` with a small quality-specific resolver. Keep the generic resolver for `output_format` and `size`.

The quality resolver accepts only `low`, `medium`, and `high`; every other request value returns `medium`.

## Tests

Update the existing top-level normalization table to verify:

- a request quality of `low`, `medium`, or `high` wins over a conflicting upstream quality;
- omitted, `auto`, whitespace-only, mixed-case, and unknown request qualities return `medium`;
- `output_format` and `size` retain upstream-first behavior;
- image generation and editing share the result;
- the raw `data` value remains unchanged.

Run the targeted OpenAI image response normalization tests and the OpenAI relay package tests.

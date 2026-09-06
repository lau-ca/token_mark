# OpenAI Image Response Quality Normalization Design

## Goal

Adjust only the `quality` field produced by the opt-in OpenAI image response normalizer. The normalized response must preserve the upstream `quality` value without interpreting or rewriting it. Only a missing upstream field is supplemented from the client request.

All other image response normalization behavior remains unchanged.

## Quality Resolution

Resolve `quality` in this order:

1. If the upstream response contains a top-level `quality` field, preserve that field exactly as returned. Do not validate, normalize, trim, or replace its value.
2. If the upstream response omits the field, copy the normalized original client `dto.ImageRequest.Quality` value stored on `RelayInfo` when it is non-empty.
3. If neither the upstream nor the client request provides a value, use `medium`.

## Unchanged Behavior

- The behavior applies only when `normalize_openai_image_response` is enabled.
- Successful non-streaming image generation and editing responses use the same logic.
- `created`, `output_format`, and `size` keep their existing upstream-first resolution.
- `usage` keeps its existing leaf-by-leaf normalization.
- `data` remains byte-for-byte untouched by the normalizer.
- Streaming responses, errors, and channels with the switch disabled remain unchanged.
- Existing response parameter overrides still run before normalization. Their resulting `quality` field is treated as the upstream response value and is preserved when present.

## Implementation

Use presence-based handling for `quality`: leave the response body untouched when the top-level field exists, and set the field only when it is absent. Keep the existing generic resolution for `output_format` and `size`.

## Tests

Update the existing top-level normalization table to verify:

- any present upstream `quality` value remains byte-for-byte unchanged at that field, including non-standard values;
- a missing upstream field is filled from the request quality;
- a missing upstream field with no request quality uses `medium`;
- `output_format` and `size` retain upstream-first behavior;
- image generation and editing share the result;
- the raw `data` value remains unchanged.

Run the targeted OpenAI image response normalization tests and the OpenAI relay package tests.

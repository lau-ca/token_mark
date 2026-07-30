# GPT-image-2 Parameter Override Integration and Response Quality Design

## Goal

Move the GPT-image-2 prompt compatibility fallback into the existing per-channel parameter override capability, and copy the client-requested image `quality` into the top-level image response when the channel contains the response operation. Both behaviors must work for image generation and image editing, and only channels configured with the corresponding override operations may change.

## Selected Approach

Extend the existing operation-based parameter override format with request and response phases. The default frontend adds one GPT-image-2 preset to the existing parameter override editor. It does not add another channel form section, model input, template input, or response switch.

The preset contains two explicit operations:

```json
{
  "operations": [
    {
      "description": "Append GPT Image size and quality to prompt",
      "phase": "request",
      "path": "prompt",
      "mode": "append_template",
      "value": "\n\nOutput image requirements: size=${body.size}; quality=${body.quality}.",
      "conditions": [
        {
          "path": "model",
          "mode": "full",
          "value": "gpt-image-2"
        }
      ],
      "logic": "AND"
    },
    {
      "description": "Copy requested image quality to response",
      "phase": "response",
      "path": "quality",
      "mode": "set_from_request",
      "from": "quality",
      "conditions": [
        {
          "path": "model",
          "mode": "full",
          "value": "gpt-image-2"
        }
      ],
      "logic": "AND"
    }
  ]
}
```

The operation names and fields are general rather than GPT-image-2-specific so the same mechanism can be reused for other channel compatibility rules without adding more channel settings.

## Alternatives Considered

### Keep the dedicated backend setting and only merge the UI

This would reduce visible controls but leave two independent configuration systems with overlapping responsibilities. Existing behavior would be harder to inspect because the parameter override editor would not be the source of truth.

### Add a separate response-quality channel switch

This is smaller on the backend but adds another channel setting immediately after the user requested fewer settings. It also creates a narrow one-off response transformation mechanism.

### Extend the existing parameter override operations

This is the selected approach. The channel keeps one configuration surface, both request and response behavior are visible as operations, and channels without the preset remain untouched.

## Operation Semantics

### Request phase

`phase: "request"` is the default for backward compatibility. Existing operations without a phase retain their current behavior.

`append_template` requires a target `path` and string `value`. Before appending, `${body.<path>}` variables are resolved against the normalized client request. For this task, the preset uses `${body.size}` and `${body.quality}`.

Rendering rules:

- The normalized client request is the variable source for both JSON generation and multipart editing.
- An absent or blank `size` or `quality` variable renders as `auto`.
- If both referenced values are absent, the operation is a no-op so a meaningless requirement is not appended.
- The rendered text is appended exactly as configured.
- If the target string already ends with the rendered value, it is not appended again.
- The structured `size` and `quality` request parameters remain unchanged and continue to be forwarded.

### Response phase

`phase: "response"` operations are not executed while constructing the upstream request. They are evaluated by the OpenAI image response path after the upstream response has been read and validated, but before it is written to the client.

`set_from_request` copies a value from the normalized original client request into the response:

- `from` identifies a normalized request field.
- `path` identifies the response destination.
- The operation is a no-op when the source field was absent or blank.
- When the source is present, it overwrites an upstream value at the same response path. The client-requested value is authoritative for this compatibility rule.
- Conditions are evaluated against the normalized original client request, not the upstream response. This allows the preset to match `model: gpt-image-2` even though the image response does not include a model field.

For the confirmed GPT-image-2 response shape, the result is at the top level:

```json
{
  "created": 1785384134,
  "background": "opaque",
  "data": [
    {
      "b64_json": ""
    }
  ],
  "output_format": "png",
  "quality": "low",
  "size": "1254x1254",
  "usage": {
    "input_tokens": 50,
    "output_tokens": 229,
    "total_tokens": 279
  }
}
```

The response body is patched in place with `sjson` so large `b64_json` values are not decoded and re-encoded.

## Generation and Editing Data Flow

### Image generation

1. Parse the client JSON into `dto.ImageRequest`.
2. Apply channel model mapping.
3. Evaluate matching request-phase template operations using the normalized request as the variable source.
4. Convert the modified request through the provider adaptor.
5. Apply the remaining existing request operations at their existing converted-request point.
6. Send the upstream request.
7. Validate the upstream image response.
8. Apply matching response-phase operations using the original client request as the source.
9. Return the patched response.

### Image editing

1. Parse multipart fields and files into the existing normalized `dto.ImageRequest` plus multipart storage.
2. Apply channel model mapping.
3. Evaluate matching request-phase template operations on the normalized request.
4. Rebuild the multipart upstream request from the modified normalized scalar fields while preserving image files, masks, and unknown extension fields.
5. Send the upstream request.
6. Use the same OpenAI image response handler and response-phase operation path as generation.

This avoids relying on raw `body` JSON for edits, because multipart editing does not place `size`, `quality`, and `prompt` in a JSON request body.

## Pass-through Behavior

If a matching request-phase operation modifies the normalized image request, raw request-body pass-through is disabled for that request so the modified prompt reaches the upstream provider. A channel with no matching operation keeps its existing pass-through behavior.

Response-only operations do not affect request pass-through.

## Channel Isolation

- Parameter overrides are loaded from the channel selected for the current relay request.
- Only operations in that channel's `param_override` are evaluated.
- The GPT-image-2 preset is not enabled by default.
- Channels without the preset have unchanged prompts and unchanged responses.
- A model condition that does not match causes both preset operations to be no-ops.
- No system-wide image setting is introduced.

## Existing Configuration Compatibility

The already-deployed `image_prompt_parameter_append` setting must not stop working immediately.

- Backend reading remains temporarily supported for channels that already contain the legacy setting.
- When loading a legacy-enabled channel in the default frontend, the equivalent request-phase operation is merged into `param_override` unless an equivalent operation already exists.
- On save, the default frontend writes the operation-based configuration and omits the legacy setting.
- The dedicated GPT Image configuration controls are removed from the default frontend.
- New configuration and documentation use only parameter override operations.
- Classic UI is outside this change.

This provides a forward migration without a database migration or a disruptive production cutover.

## Error Handling

- Unknown request template variables produce a channel parameter override error instead of being sent literally.
- `append_template` rejects non-string target values and non-string templates.
- `set_from_request` rejects missing `from` or `path` fields.
- Invalid response patching returns the existing bad-response error before partial output is written.
- A missing response source value is not an error; the response remains unchanged.
- Operation failures retain the existing skip-retry behavior for invalid channel parameter override configuration.

## Frontend

The default channel editor keeps the existing Parameter Override field and dialog. Changes are limited to that dialog:

- Add `Request template append` and `Set response from request` operation modes.
- Add the request/response phase selector when required by the selected mode.
- Add a `GPT-image-2 size, quality compatibility` preset containing both operations.
- Keep raw JSON editing available for the full operation object.
- Remove the dedicated GPT Image switch, model input, template textarea, and associated help text from the channel drawer.
- Add all new user-facing strings to every supported default-frontend locale.

## Testing

Backend tests protect these observable contracts:

- A generation request appends the requested `size` and `quality` to `prompt` when the channel preset matches.
- A multipart editing request appends the same values and preserves images, masks, and unknown form fields.
- Missing values render according to the documented rules.
- Duplicate rendered suffixes are not appended.
- The response `quality` is written at the top level from the original client request for generation and editing.
- A client-requested `quality` overwrites a conflicting upstream `quality`.
- An absent client `quality` leaves the upstream response unchanged.
- Unmatched models and channels without the operations remain unchanged.
- Existing request-only parameter override operations retain their current behavior.
- Legacy `image_prompt_parameter_append` channel configuration still works during migration.

Frontend tests protect preset generation, existing-channel hydration, legacy migration, serialization, and removal of the dedicated form fields. Verification includes targeted Go tests, frontend form tests, TypeScript checks, lint of changed files, and a complete `web/default` production build. Browser end-to-end testing is outside this task unless explicitly requested.

## Acceptance Criteria

- The default channel UI shows no separate GPT Image prompt-append configuration block.
- An administrator can add both compatibility behaviors from the existing parameter override preset list.
- Image generation and image editing both append the normalized request `size` and `quality` to the prompt.
- Image generation and image editing both return the explicitly requested `quality` at the response top level.
- Only the configured channel and matching model are affected.
- Existing configured channels continue working through the compatibility migration.
- The full current source tree can be built and deployed together after implementation.

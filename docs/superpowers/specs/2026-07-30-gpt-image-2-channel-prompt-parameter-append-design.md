# GPT-image-2 Channel Prompt Parameter Append Design

## Goal

Add an opt-in channel capability that appends the requested image `size` and `quality` to the image prompt for upstream channels that accept those fields but do not reliably honor them. The original parameters remain in the upstream request. The capability must work for both `/v1/images/generations` and `/v1/images/edits` and must affect only channels that explicitly enable it.

## Scope

- Add a dedicated per-channel configuration instead of extending the generic parameter override language.
- Match configured models after channel model mapping so aliases mapped to `gpt-image-2` are supported.
- Support JSON image-generation requests and multipart image-edit requests.
- Expose the configuration in the default channel-management frontend.
- Preserve billing inputs, files, masks, and unrelated request fields.

The change does not attempt to make an upstream provider honor `size` or `quality` as structured parameters. It adds a prompt-based compatibility fallback while continuing to forward the structured parameters.

## Channel Configuration

Add the following optional channel setting:

```json
{
  "image_prompt_parameter_append": {
    "enabled": true,
    "models": ["gpt-image-2"],
    "template": "Output image requirements: size={{size}}; quality={{quality}}."
  }
}
```

Fields:

- `enabled`: Enables the capability for this channel.
- `models`: Exact upstream model names for which the capability applies. An empty list uses the safe default `gpt-image-2`.
- `template`: Text appended to the image prompt. The only supported placeholders are `{{size}}` and `{{quality}}`. An empty template uses the default shown above.

The narrow placeholder set is intentional. It avoids introducing a general-purpose template language into channel configuration.

## Request Processing

The append operation runs after channel model mapping and before provider request conversion:

1. Read the normalized `dto.ImageRequest` produced from either JSON or multipart input.
2. Complete channel model mapping.
3. Check whether the selected channel enabled `image_prompt_parameter_append`.
4. Match the mapped upstream model against the configured model list.
5. Render the configured template from the normalized request's `Size` and `Quality` fields.
6. Append the rendered text to `Prompt` with a blank-line separator.
7. Convert and send the request through the existing provider adaptor.

The normalized request is the common source for both endpoints. Raw JSON-body variables are not used because `/v1/images/edits` normally arrives as multipart form data.

## Rendering Rules

- If both `size` and `quality` are absent, leave the prompt unchanged.
- If one value is absent, render that placeholder as `auto` and preserve the explicitly supplied value for the other placeholder.
- Trim surrounding whitespace from configured model names, the template, `size`, and `quality` before use.
- Append exactly one blank line followed by the rendered template.
- If the prompt already ends with the exact rendered template, do not append it again.
- Do not modify the structured `size` or `quality` fields.

Example input:

```json
{
  "model": "gpt-image-2",
  "prompt": "Create a futuristic city at night",
  "size": "2048x1152",
  "quality": "high"
}
```

Resulting upstream prompt:

```text
Create a futuristic city at night

Output image requirements: size=2048x1152; quality=high.
```

## Multipart Image Editing

The OpenAI image-edit adaptor currently applies parameter overrides to a normalized `ImageRequest`, then copies most non-file fields from the original multipart form. That behavior would discard the modified prompt.

For multipart edits, standard normalized fields must be written from the modified `ImageRequest`, while file data and unrecognized extension fields continue to come from the parsed multipart form:

- Write `model`, `prompt`, `size`, `quality`, `n`, and other modeled scalar fields from the normalized request when present.
- Do not copy the original versions of those fields from `MultipartForm.Value`.
- Preserve `image`, `image[]`, indexed image fields, and `mask` files without decoding or re-encoding them.
- Preserve unrecognized non-file form fields.

This makes the new prompt append behavior reliable and also makes existing parameter overrides consistent for multipart edits.

## Request Body Pass-through

Raw request-body pass-through cannot apply normalized prompt modifications. For a matched image request where this capability is enabled, normal image conversion takes precedence over request-body pass-through. Pass-through behavior remains unchanged for unmatched models and channels where the capability is disabled.

The channel-management UI must explain this precedence next to the new setting.

## Frontend

Add the configuration to the `web/default` channel editor:

- Switch: `Append image parameters to prompt`
- Model list input, defaulting to `gpt-image-2`
- Template textarea with the default template
- Help text explaining that the setting is a compatibility fallback for image generation and editing, preserves the original parameters, and takes precedence over raw request-body pass-through for matched image requests

All new user-facing text must use the existing i18n systems. The default frontend must provide translations for all supported locales through the existing synchronization workflow.

## Compatibility and Safety

- Existing channels remain unchanged because the setting is disabled by default.
- The capability applies only to image relay modes.
- Model matching uses the mapped upstream model rather than the client-facing alias.
- The prompt remains user-controlled; inserting `size` and `quality` does not create a new authorization boundary.
- Billing continues to use the original request parameters and existing quota paths.
- Structured `size` and `quality` parameters continue to be sent so compliant upstream channels retain their native behavior.

## Error Handling

- Invalid channel-setting JSON continues to use the existing channel-settings validation path.
- Unknown template placeholders are rejected when saving or loading the configuration instead of being silently forwarded.
- An enabled configuration with an empty model list or template uses the documented defaults.
- Failure to render a validated template is treated as a channel configuration error and must not silently send a partially rendered prompt.

## Tests

Backend regression tests must cover:

- Disabled configuration leaves generation and editing prompts unchanged.
- Enabled configuration appends `size` and `quality` to generation prompts.
- Enabled configuration appends the same values to multipart edit prompts.
- Model aliases work after mapping to `gpt-image-2`.
- Unmatched upstream models remain unchanged.
- One missing parameter renders as `auto`; both missing parameters cause no append.
- Duplicate rendered suffixes are not appended twice.
- Multipart edits send the modified prompt and preserve image and mask files.
- Multipart edits preserve unrecognized non-file form fields.
- Matched requests use normal conversion instead of raw pass-through.
- Unknown template placeholders are rejected.

Frontend verification must cover configuration serialization, existing-channel hydration, validation, and the default frontend production build. Browser end-to-end testing is outside this task unless requested separately.

## Acceptance Criteria

- An administrator can enable the capability independently on any channel.
- A request mapped to a configured `gpt-image-2` channel sends the original `size` and `quality` fields and a prompt containing those values.
- The behavior is identical for image generation and multipart image editing.
- Disabled and unmatched channels preserve their current behavior.
- Existing request fields and edit files are preserved.
- Backend tests and frontend production builds pass.

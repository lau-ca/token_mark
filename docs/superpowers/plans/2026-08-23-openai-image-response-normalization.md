# OpenAI Image Response Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in channel switch that normalizes successful non-streaming OpenAI-compatible image responses without changing the upstream `data` value.

**Architecture:** Store the switch in the existing per-channel `settings` JSON and expose it in the default channel editor. In the shared OpenAI image JSON handler, inspect scalar paths with `gjson`, construct the fixed usage object with project JSON wrappers, and patch only top-level fields with `sjson`; streaming paths remain untouched.

**Tech Stack:** Go 1.22, Gin, `gjson`, `sjson`, Testify, React 19, TypeScript, Zod, React Hook Form, i18next, Bun/Vitest.

---

## File Structure

- Modify `relaykit/dto/channel_settings.go`: define the persisted channel switch in the shared channel settings DTO.
- Modify `relay/channel/openai/relay_image.go`: add the non-streaming normalization boundary and helper logic.
- Modify `relay/channel/openai/image_stream_test.go`: protect response precedence, leaf-level usage mapping, data preservation, and stream isolation.
- Modify `web/src/features/channels/lib/channel-form.ts`: add schema/default/hydration/serialization support.
- Modify `web/src/features/channels/types.ts`: expose the settings field in frontend types.
- Modify `web/src/features/channels/lib/channel-form-errors.ts`: include the new form field in the channel error-key registry.
- Modify `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`: render the switch and include it in advanced-setting detection.
- Create `web/src/features/channels/lib/__tests__/image-response-normalization.test.ts`: test form hydration and serialization without coupling to drawer internals.
- Modify `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`: add the switch label and description.

### Task 1: Add the backend channel setting and failing normalization tests

**Files:**
- Modify: `relaykit/dto/channel_settings.go`
- Modify: `relay/channel/openai/image_stream_test.go`

- [ ] **Step 1: Add the channel setting field**

Add this field beside the existing image response setting:

```go
NormalizeOpenAIImageResponse bool `json:"normalize_openai_image_response,omitempty"`
```

- [ ] **Step 2: Add a test fixture that enables normalization**

Use the existing `newImageTestContext` helper and configure:

```go
info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{
    NormalizeOpenAIImageResponse: true,
}
info.Request = &dto.ImageRequest{
    OutputFormat: json.RawMessage(`"webp"`),
    Quality:      "high",
    Size:         "1536x1024",
}
```

Import `encoding/json` only for the `json.RawMessage` request type; do not use it for marshal or unmarshal operations.

- [ ] **Step 3: Add failing table tests for top-level precedence and `data` preservation**

Cover both generation and editing relay modes. Use an upstream body containing provider-specific `data` fields and assert its raw `gjson.GetBytes(body, "data").Raw` value equals the output `data` raw value. Assert:

```json
{
  "created": 1787414536,
  "output_format": "png",
  "quality": "medium",
  "size": "3840x2160"
}
```

remains authoritative when present upstream, and request values fill each missing field independently.

- [ ] **Step 4: Add failing usage leaf tests**

Use deterministic cases for:

```json
{"usage":{"input_tokens":60,"output_tokens":1756,"total_tokens":1816,"input_tokens_details":{"text_tokens":60}}}
```

```json
{"usage":{"prompt_tokens":60,"completion_tokens":1756,"total_tokens":1816,"prompt_tokens_details":{"text_tokens":60,"image_tokens":4},"completion_tokens_details":{"text_tokens":2,"image_tokens":1754}}}
```

```json
{"usage":{"input_tokens":0,"prompt_tokens":99,"input_tokens_details":{"text_tokens":0},"prompt_tokens_details":{"text_tokens":88,"image_tokens":7}}}
```

Assert every standard leaf exists, explicit primary zero wins, missing siblings are zero, and provider-only fields are absent from the final `usage` object.

- [ ] **Step 5: Add failing invalid-value and disabled-switch tests**

Cover negative, fractional, string, object, and overflowing values. Assert fallback aliases are used only when valid; otherwise the leaf is zero. Reuse the existing no-operation response test to assert a disabled channel returns the exact original JSON body.

- [ ] **Step 6: Run the focused backend tests and verify failure**

Run:

```bash
go test ./relay/channel/openai -run 'TestOpenaiImageHandlerNormalizes|TestOpenaiImageHandlerLeaves' -count=1
```

Expected: FAIL because the normalizer and/or new response fields are not implemented.

### Task 2: Implement non-streaming response normalization

**Files:**
- Modify: `relay/channel/openai/relay_image.go`

- [ ] **Step 1: Gate normalization in `OpenaiImageHandler`**

After URL stripping and response parameter overrides, call:

```go
responseBody, err = normalizeOpenAIImageResponse(responseBody, info)
if err != nil {
    return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
}
```

The helper must immediately return the input bytes unchanged unless `info.ChannelOtherSettings.NormalizeOpenAIImageResponse` is true. Do not call it from `OpenaiImageStreamHandler` or `openaiImageJSONAsStreamHandler`.

- [ ] **Step 2: Define fixed normalized usage types**

Keep the types private to `relay_image.go`:

```go
type normalizedImageTokenDetails struct {
    ImageTokens int `json:"image_tokens"`
    TextTokens  int `json:"text_tokens"`
}

type normalizedImageUsage struct {
    InputTokens         int                         `json:"input_tokens"`
    InputTokensDetails  normalizedImageTokenDetails `json:"input_tokens_details"`
    OutputTokens        int                         `json:"output_tokens"`
    TotalTokens         int                         `json:"total_tokens"`
    OutputTokensDetails normalizedImageTokenDetails `json:"output_tokens_details"`
}
```

- [ ] **Step 3: Resolve request-derived strings**

Read `info.Request.(*dto.ImageRequest)` safely. Decode `OutputFormat` through `common.Unmarshal` into a string. Resolve fields with upstream-first precedence:

```go
outputFormat := firstValidImageString(responseBody, "output_format", requestOutputFormat, "png")
quality := firstValidImageString(responseBody, "quality", request.Quality, "auto")
size := firstValidImageString(responseBody, "size", request.Size, "auto")
```

Treat absent, JSON null, non-string, and blank strings as unusable.

- [ ] **Step 4: Resolve integer usage leaves independently**

Use a private function with this contract:

```go
func imageUsageInt(body []byte, primaryPath string, fallbackPath string) int
```

Inspect the `gjson.Result` type and raw number. Accept only non-negative integral JSON numbers within `int` range. An explicit primary zero is valid. If primary is absent or invalid, try the fallback. Return zero if neither is valid.

Build `normalizedImageUsage` from the seven mappings in the design document. Do not calculate `total_tokens`.

- [ ] **Step 5: Patch the response without touching `data`**

Reject a non-object JSON root. Marshal only the fixed usage struct with `common.Marshal`, then patch:

```go
body, err = sjson.SetBytes(body, "created", created)
body, err = sjson.SetBytes(body, "output_format", outputFormat)
body, err = sjson.SetBytes(body, "quality", quality)
body, err = sjson.SetBytes(body, "size", size)
body, err = sjson.SetRawBytes(body, "usage", usageJSON)
```

Use the valid upstream numeric `created` value when present; otherwise use `time.Now().Unix()`.

- [ ] **Step 6: Run and pass focused backend tests**

Run:

```bash
go test ./relay/channel/openai -count=1
```

Expected: PASS.

- [ ] **Step 7: Verify relaykit independence**

Run:

```bash
cd relaykit && GOWORK=off go build ./...
```

Expected: PASS.

### Task 3: Add frontend setting persistence and tests

**Files:**
- Modify: `web/src/features/channels/lib/channel-form.ts`
- Modify: `web/src/features/channels/types.ts`
- Modify: `web/src/features/channels/lib/channel-form-errors.ts`
- Create: `web/src/features/channels/lib/__tests__/image-response-normalization.test.ts`

- [ ] **Step 1: Write failing form tests**

Create tests that call `transformChannelToFormDefaults`, `transformFormDataToCreatePayload`, and `transformFormDataToUpdatePayload`. Build a minimal type-1 channel with:

```ts
settings: JSON.stringify({ normalize_openai_image_response: true })
```

Assert hydration returns `normalize_openai_image_response: true`, serialization writes the key for type 1, and serialization removes it for an unrelated channel type.

- [ ] **Step 2: Run the form test and verify failure**

Run:

```bash
cd web && bun test src/features/channels/lib/__tests__/image-response-normalization.test.ts
```

Expected: FAIL because the form field is not defined or serialized.

- [ ] **Step 3: Add schema, default, type, and hydration support**

Add:

```ts
normalize_openai_image_response: z.boolean().optional()
```

and default it to `false`. Parse it with strict boolean semantics:

```ts
normalizeOpenAIImageResponse =
  parsed.normalize_openai_image_response === true
```

Return it from `transformChannelToFormDefaults`, add it to `ChannelOtherSettings`, and add the field name to the form error-key list.

- [ ] **Step 4: Serialize only for supported channel types**

Inside the same channel-type branch as `force_image_b64_json_no_url`, write:

```ts
settingsObj.normalize_openai_image_response =
  formData.normalize_openai_image_response === true
```

Delete the key in the unsupported-type branch.

- [ ] **Step 5: Run and pass the form test**

Run:

```bash
cd web && bun test src/features/channels/lib/__tests__/image-response-normalization.test.ts
```

Expected: PASS.

### Task 4: Add the channel editor switch and translations

**Files:**
- Modify: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`

- [ ] **Step 1: Register the field in drawer state detection**

Add `normalize_openai_image_response` to `FORM_ONLY_FIELDS` and include `values.normalize_openai_image_response` in `hasAdvancedSettingsValues`.

- [ ] **Step 2: Render the switch beside image URL hiding**

Use the existing `FormField`/`Switch` pattern:

```tsx
<FormField
  control={form.control}
  name='normalize_openai_image_response'
  render={({ field }) => (
    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
      <div className='space-y-0.5'>
        <FormLabel className='text-sm'>
          {t('Normalize OpenAI image responses')}
        </FormLabel>
        <FormDescription>
          {t(
            'Fill standard fields for non-streaming image generation and edits without changing data'
          )}
        </FormDescription>
      </div>
      <FormControl>
        <Switch checked={field.value} onCheckedChange={field.onChange} />
      </FormControl>
    </FormItem>
  )}
/>
```

- [ ] **Step 3: Add all locale values**

Add both flat English keys to all seven locale files. Use concise native translations and retain the English keys exactly.

- [ ] **Step 4: Run i18n and frontend checks**

Run:

```bash
cd web && bun run i18n:sync
cd web && bun test src/features/channels/lib/__tests__/image-response-normalization.test.ts
cd web && bun run typecheck
cd web && bun run lint -- src/features/channels/lib/channel-form.ts src/features/channels/types.ts src/features/channels/lib/channel-form-errors.ts src/features/channels/components/drawers/channel-mutate-drawer.tsx src/features/channels/lib/__tests__/image-response-normalization.test.ts
```

Expected: all commands PASS with no missing translation keys or lint errors in changed files.

### Task 5: Final verification and scope audit

**Files:**
- Verify all files changed by Tasks 1-4.

- [ ] **Step 1: Run backend regression tests**

Run:

```bash
go test ./relay/channel/openai -count=1
```

Expected: PASS.

- [ ] **Step 2: Run frontend production build**

Run:

```bash
cd web && bun run build
```

Expected: PASS.

- [ ] **Step 3: Inspect the final diff**

Run:

```bash
git diff --check
git status --short
git diff -- relaykit/dto/channel_settings.go relay/channel/openai/relay_image.go relay/channel/openai/image_stream_test.go web/src/features/channels web/src/i18n/locales
```

Confirm only the requested setting, normalization path, tests, and translations were changed. Do not stage or commit unrelated pre-existing worktree changes.

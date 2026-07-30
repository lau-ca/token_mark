# GPT-image-2 Channel Prompt Parameter Append Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in per-channel setting that appends normalized image `size` and `quality` values to prompts sent through both image generation and multipart image editing.

**Architecture:** Store a narrow `image_prompt_parameter_append` object in the existing channel `setting` JSON. Keep validation and rendering behavior on the configuration type, apply it after model mapping in `ImageHelper`, and force normal request conversion only when a prompt was modified. Update the OpenAI multipart adaptor to write the normalized prompt and quality values instead of replaying their original form values.

**Tech Stack:** Go 1.22+, Gin, testify, React 19, TypeScript, React Hook Form, Zod, Base UI/Tailwind, React 18/Semi Design, i18next, Bun

---

## File Structure

- `dto/channel_settings.go`: Define and validate the channel configuration; render the narrow `{{size}}` and `{{quality}}` template.
- `dto/channel_settings_test.go`: Protect defaults, matching, rendering, duplicate prevention, and placeholder validation.
- `model/channel.go`: Invoke the new channel-setting validation during create/update validation.
- `relay/image_handler.go`: Apply the prompt append after model mapping and control pass-through precedence.
- `relay/image_handler_test.go`: Protect relay application, model matching, and pass-through behavior.
- `relay/channel/openai/adaptor.go`: Rebuild multipart edits using the normalized prompt, size, and quality.
- `relay/channel/openai/image_edit_test.go`: Protect modified multipart fields and preserved extension fields/files.
- `web/default/src/features/channels/lib/channel-form.ts`: Add form schema, defaults, hydration, and serialization.
- `web/default/src/features/channels/types.ts`: Add the serialized setting type.
- `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`: Add the switch, models input, and template textarea.
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`: Add channel-setting translations.
- `web/classic/src/components/table/channels/modals/EditChannelModal.jsx`: Add state hydration, serialization, cleanup, and controls.
- `web/classic/src/i18n/locales/{en,fr,ja,ru,vi,zh-CN,zh-TW,zh}.json`: Add channel-setting translations.

### Task 1: Backend configuration contract

**Files:**
- Modify: `dto/channel_settings.go`
- Modify: `dto/channel_settings_test.go`
- Modify: `model/channel.go`

- [ ] **Step 1: Write failing configuration tests**

Add deterministic table tests that construct `ImagePromptParameterAppendConfig` values and assert:

```go
func TestImagePromptParameterAppendConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     ImagePromptParameterAppendConfig
		model      string
		prompt     string
		size       string
		quality    string
		wantPrompt string
		wantApply  bool
		wantErr    string
	}{
		{
			name:       "renders configured values",
			config:     ImagePromptParameterAppendConfig{Enabled: true, Models: []string{"gpt-image-2"}},
			model:      "gpt-image-2",
			prompt:     "draw a cat",
			size:       "2048x1152",
			quality:    "high",
			wantPrompt: "draw a cat\n\nOutput image requirements: size=2048x1152; quality=high.",
			wantApply:  true,
		},
		{
			name:       "uses auto for one missing value",
			config:     ImagePromptParameterAppendConfig{Enabled: true},
			model:      "gpt-image-2",
			prompt:     "draw a cat",
			size:       "1024x1024",
			wantPrompt: "draw a cat\n\nOutput image requirements: size=1024x1024; quality=auto.",
			wantApply:  true,
		},
		{
			name:      "rejects unknown placeholder",
			config:    ImagePromptParameterAppendConfig{Enabled: true, Template: "size={{size}} style={{style}}"},
			model:     "gpt-image-2",
			prompt:    "draw a cat",
			size:      "1024x1024",
			wantErr:   "unsupported placeholder: style",
		},
	}
}
```

Also cover disabled configuration, unmatched model, empty size and quality, trimmed model names, an empty model list defaulting to `gpt-image-2`, an empty template defaulting to the documented template, and a prompt already ending in the rendered suffix.

- [ ] **Step 2: Run the tests and verify failure**

Run:

```bash
go test ./dto -run TestImagePromptParameterAppendConfig -count=1
```

Expected: compilation failure because `ImagePromptParameterAppendConfig` does not exist.

- [ ] **Step 3: Implement the configuration type and renderer**

Add the setting and narrow renderer:

```go
const DefaultImagePromptParameterAppendTemplate = "Output image requirements: size={{size}}; quality={{quality}}."

type ImagePromptParameterAppendConfig struct {
	Enabled  bool     `json:"enabled,omitempty"`
	Models   []string `json:"models,omitempty"`
	Template string   `json:"template,omitempty"`
}

type ChannelSettings struct {
	ForceFormat               bool                              `json:"force_format,omitempty"`
	ThinkingToContent         bool                              `json:"thinking_to_content,omitempty"`
	Proxy                     string                            `json:"proxy"`
	PassThroughBodyEnabled    bool                              `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt              string                            `json:"system_prompt,omitempty"`
	SystemPromptOverride      bool                              `json:"system_prompt_override,omitempty"`
	ImagePromptParameterAppend *ImagePromptParameterAppendConfig `json:"image_prompt_parameter_append,omitempty"`
}
```

Implement methods with these signatures:

```go
func (c *ImagePromptParameterAppendConfig) Validate() error
func (c *ImagePromptParameterAppendConfig) MatchesModel(model string) bool
func (c *ImagePromptParameterAppendConfig) Append(prompt, size, quality string) (string, bool, error)
```

Validation must accept only `{{size}}` and `{{quality}}`. `Append` must return the original prompt when disabled, when both parameters are absent, or when the exact rendered suffix is already present. Use `auto` only for the missing member of a partially specified pair.

- [ ] **Step 4: Validate the setting during channel validation**

In `Channel.ValidateSettings`, add:

```go
if channelParams.ImagePromptParameterAppend != nil {
	if err := channelParams.ImagePromptParameterAppend.Validate(); err != nil {
		return fmt.Errorf("image_prompt_parameter_append: %w", err)
	}
}
```

- [ ] **Step 5: Run backend configuration tests**

Run:

```bash
go test ./dto ./model -run 'TestImagePromptParameterAppendConfig|Test.*ValidateSettings' -count=1
```

Expected: PASS.

### Task 2: Relay application and pass-through precedence

**Files:**
- Modify: `relay/image_handler.go`
- Modify: `relay/image_handler_test.go`

- [ ] **Step 1: Write failing relay tests**

Add tests around a focused helper:

```go
func TestApplyImagePromptParameterAppend(t *testing.T) {
	request := &dto.ImageRequest{
		Model:   "gpt-image-2",
		Prompt:  "draw a cat",
		Size:    "2048x1152",
		Quality: "high",
	}
	info := &relaycommon.RelayInfo{
		ChannelSetting: dto.ChannelSettings{
			ImagePromptParameterAppend: &dto.ImagePromptParameterAppendConfig{Enabled: true},
		},
	}

	applied, err := applyImagePromptParameterAppend(info, request)
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, "draw a cat\n\nOutput image requirements: size=2048x1152; quality=high.", request.Prompt)
}
```

Add unmatched-model and disabled-setting cases. Extend pass-through tests so a modified prompt forces normal conversion while an unmatched request preserves existing pass-through behavior.

- [ ] **Step 2: Run the tests and verify failure**

Run:

```bash
go test ./relay -run 'TestApplyImagePromptParameterAppend|TestCompositeImageRequestDisablesBodyPassthrough' -count=1
```

Expected: compilation failure because the helper and new pass-through argument do not exist.

- [ ] **Step 3: Apply the setting after model mapping**

Immediately after `ModelMappedHelper`, call:

```go
promptParametersAppended, err := applyImagePromptParameterAppend(info, request)
if err != nil {
	return types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
}
```

Implement:

```go
func applyImagePromptParameterAppend(info *relaycommon.RelayInfo, request *dto.ImageRequest) (bool, error) {
	if info == nil || request == nil || info.ChannelSetting.ImagePromptParameterAppend == nil {
		return false, nil
	}
	config := info.ChannelSetting.ImagePromptParameterAppend
	if !config.MatchesModel(request.Model) {
		return false, nil
	}
	prompt, applied, err := config.Append(request.Prompt, request.Size, request.Quality)
	if err != nil {
		return false, err
	}
	if applied {
		request.Prompt = prompt
	}
	return applied, nil
}
```

- [ ] **Step 4: Give prompt modification precedence over pass-through**

Change the helper signature and call site:

```go
func shouldPassThroughImageRequest(c *gin.Context, channelPassThroughEnabled, requestModified bool) bool {
	if requestModified || common.GetContextKeyBool(c, constant.ContextKeyCompositeDisableRequestBodyPassthrough) {
		return false
	}
	return model_setting.GetGlobalSettings().PassThroughRequestEnabled || channelPassThroughEnabled
}
```

- [ ] **Step 5: Run relay tests**

Run:

```bash
go test ./relay -run 'TestApplyImagePromptParameterAppend|TestCompositeImageRequestDisablesBodyPassthrough' -count=1
```

Expected: PASS.

### Task 3: Multipart edit consistency

**Files:**
- Modify: `relay/channel/openai/adaptor.go`
- Modify: `relay/channel/openai/image_edit_test.go`

- [ ] **Step 1: Extend the multipart override regression test**

Add a prompt append and quality override to the existing parameter-override test:

```go
map[string]interface{}{
	"path":  "prompt",
	"mode":  "append",
	"value": "\n\nOutput image requirements: size=2048x1152; quality=high.",
},
map[string]interface{}{
	"path":  "quality",
	"mode":  "set",
	"value": "high",
},
```

Assert the replayed form contains the modified prompt and quality, the normalized size, an untouched custom form field, and the original image bytes.

- [ ] **Step 2: Run the test and verify failure**

Run:

```bash
go test ./relay/channel/openai -run TestConvertImageEditRequestMultipartAppliesParamOverride -count=1
```

Expected: FAIL because prompt and quality still come from the original multipart form.

- [ ] **Step 3: Rebuild standard fields from the normalized request**

When copying `MultipartForm.Value`, skip normalized fields that must be written from `request`:

```go
switch key {
case "model", "group", "prompt", "size", "quality":
	continue
case "response_format":
	if shouldForceChannelImageResponseFormat(info) {
		continue
	}
}
```

Then write:

```go
writer.WriteField("model", request.Model)
writer.WriteField("prompt", request.Prompt)
if request.Size != "" {
	writer.WriteField("size", request.Size)
}
if request.Quality != "" {
	writer.WriteField("quality", request.Quality)
}
```

Continue copying unrecognized non-file fields and all existing image/mask files through the current code path.

- [ ] **Step 4: Run OpenAI image adaptor tests**

Run:

```bash
go test ./relay/channel/openai -run 'TestConvertImage(Edit|Generation)Request' -count=1
```

Expected: PASS.

### Task 4: Default frontend channel configuration

**Files:**
- Modify: `web/default/src/features/channels/lib/channel-form.ts`
- Modify: `web/default/src/features/channels/types.ts`
- Modify: `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`

- [ ] **Step 1: Add schema fields and serialized type**

Add flat form fields:

```ts
image_prompt_parameter_append_enabled: z.boolean().optional(),
image_prompt_parameter_append_models: z.string().optional(),
image_prompt_parameter_append_template: z.string().optional(),
```

Add the serialized type:

```ts
image_prompt_parameter_append?: {
  enabled?: boolean
  models?: string[]
  template?: string
}
```

- [ ] **Step 2: Add defaults, hydration, and serialization**

Use these defaults:

```ts
image_prompt_parameter_append_enabled: false,
image_prompt_parameter_append_models: 'gpt-image-2',
image_prompt_parameter_append_template:
  'Output image requirements: size={{size}}; quality={{quality}}.',
```

Hydrate from `parsed.image_prompt_parameter_append`. Serialize a normalized, deduplicated comma-separated model input to `models: string[]`. Include the object only when enabled, so disabled channels preserve compact setting JSON.

- [ ] **Step 3: Add channel controls**

Add a switch to the existing extra-settings list. When enabled, show an input and textarea beneath it:

```tsx
<FormField
  control={form.control}
  name='image_prompt_parameter_append_enabled'
  render={({ field }) => (
    <FormItem className='flex items-center justify-between px-4 py-3'>
      <div className='space-y-0.5'>
        <FormLabel>{t('Append image parameters to prompt')}</FormLabel>
        <FormDescription>
          {t('Use size and quality as a prompt fallback for selected image models')}
        </FormDescription>
      </div>
      <FormControl>
        <Switch checked={field.value} onCheckedChange={field.onChange} />
      </FormControl>
    </FormItem>
  )}
/>
```

Add help text stating that structured parameters remain present and matched requests use normal conversion instead of raw pass-through.

- [ ] **Step 4: Format and validate default frontend code**

Run:

```bash
cd web/default
bun run format -- src/features/channels/lib/channel-form.ts src/features/channels/types.ts src/features/channels/components/drawers/channel-mutate-drawer.tsx
bun run typecheck
bun run lint
```

Expected: all commands exit successfully.

### Task 5: Classic frontend channel configuration

**Files:**
- Modify: `web/classic/src/components/table/channels/modals/EditChannelModal.jsx`

- [ ] **Step 1: Add defaults and hydration**

Add the same three flat form fields to `originInputs`, the `channelSettings` state, parsed setting hydration, reset paths, and edit-form initialization.

- [ ] **Step 2: Serialize and clean temporary fields**

Build the nested configuration only when enabled:

```js
image_prompt_parameter_append: localInputs.image_prompt_parameter_append_enabled
  ? {
      enabled: true,
      models: String(localInputs.image_prompt_parameter_append_models || 'gpt-image-2')
        .split(',')
        .map((model) => model.trim())
        .filter(Boolean),
      template:
        localInputs.image_prompt_parameter_append_template ||
        'Output image requirements: size={{size}}; quality={{quality}}.',
    }
  : undefined,
```

Delete the three temporary flat fields before submission.

- [ ] **Step 3: Add Semi Design controls**

Add a switch, model input, and template textarea next to existing channel extra settings. Only show the input and textarea when enabled. Wrap every user-facing string in `t()`.

- [ ] **Step 4: Format and lint the classic frontend change**

Run:

```bash
cd web/classic
bunx prettier src/components/table/channels/modals/EditChannelModal.jsx --write
bunx eslint src/components/table/channels/modals/EditChannelModal.jsx
```

Expected: both commands exit successfully.

### Task 6: Internationalization and final verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify: `web/classic/src/i18n/locales/en.json`
- Modify: `web/classic/src/i18n/locales/fr.json`
- Modify: `web/classic/src/i18n/locales/ja.json`
- Modify: `web/classic/src/i18n/locales/ru.json`
- Modify: `web/classic/src/i18n/locales/vi.json`
- Modify: `web/classic/src/i18n/locales/zh-CN.json`
- Modify: `web/classic/src/i18n/locales/zh-TW.json`
- Modify: `web/classic/src/i18n/locales/zh.json`

- [ ] **Step 1: Add translations**

Add translations for:

```text
Append image parameters to prompt
Use size and quality as a prompt fallback for selected image models
Applicable upstream models
Separate multiple models with commas
Prompt append template
Available variables: {{size}} and {{quality}}
Structured parameters remain unchanged; matched image requests use normal conversion instead of raw request-body pass-through
```

- [ ] **Step 2: Run i18n synchronization**

Run:

```bash
cd web/default && bun run i18n:sync
cd ../classic && bun run i18n:sync
```

Expected: no missing keys for the new strings. Inspect generated reports and keep only changes required by the new keys.

- [ ] **Step 3: Run targeted Go tests**

Run:

```bash
go test ./dto ./model ./relay ./relay/channel/openai -count=1
```

Expected: PASS.

- [ ] **Step 4: Run frontend production checks**

Run:

```bash
cd web/default && bun run build:check
cd ../classic && bun run build
```

Expected: both production builds succeed.

- [ ] **Step 5: Review the final diff**

Run:

```bash
git diff --check
git status --short
git diff -- dto/channel_settings.go dto/channel_settings_test.go model/channel.go relay/image_handler.go relay/image_handler_test.go relay/channel/openai/adaptor.go relay/channel/openai/image_edit_test.go web/default/src/features/channels web/default/src/i18n/locales web/classic/src/components/table/channels/modals/EditChannelModal.jsx web/classic/src/i18n/locales
```

Expected: no whitespace errors, no unrelated files included, and every design acceptance criterion represented in code or tests.

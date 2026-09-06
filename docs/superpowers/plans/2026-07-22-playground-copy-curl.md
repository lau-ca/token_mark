# Playground API Integration Guide Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the playground’s single curl copier with a developer-facing API integration guide whose interface templates can be configured in the existing model capability metadata.

**Architecture:** Extend the existing endpoint `playground` JSON with an optional validated `integration` object, add built-in templates beside the current capability templates, and resolve those templates with the user’s selected model and example values. Reuse the existing model capability drawer for administration and render a shared integration dialog from the existing playground input toolbar. No database migration or message-storage change is required.

**Tech Stack:** Go 1.22, Gin DTO validation, React 19, TypeScript, Base UI/Sheet/Dialog primitives, Tailwind CSS, i18next, Bun tests.

**Repository constraint:** Do not create Git commits unless the user explicitly requests them. Commit steps are intentionally omitted.

---

## File map

- `dto/model_playground.go`: backend integration-template DTOs and validation.
- `dto/model_playground_test.go`: parsing and validation regression tests.
- `web/default/src/features/models/lib/model-capabilities.ts`: frontend template types, built-in templates, parsing, fallback, and serialization.
- `web/default/src/features/models/lib/model-capabilities.test.ts`: frontend template resolver tests.
- `web/default/src/features/models/components/drawers/model-capabilities-drawer.tsx`: administrator integration-template editor and preview.
- `web/default/src/features/playground/types.ts`: integration-guide response and rendering types.
- `web/default/src/features/playground/lib/curl/playground-integration-guide.ts`: safe template-variable resolution.
- `web/default/src/features/playground/lib/curl/playground-integration-guide.test.ts`: guide-resolution and video two-interface tests.
- `web/default/src/features/playground/components/input/playground-api-integration-button.tsx`: shared toolbar trigger.
- `web/default/src/features/playground/components/input/playground-api-integration-dialog.tsx`: structured developer guide UI.
- `web/default/src/features/playground/components/input/playground-input.tsx`: chat guide inputs.
- `web/default/src/features/playground/components/input/playground-input-tools.tsx`: chat trigger placement.
- `web/default/src/features/playground/components/media/playground-media-input.tsx`: image/video guide inputs.
- `web/default/src/features/playground/lib/index.ts`: exports.
- `web/default/src/i18n/locales/*.json`: translated labels, written through the project i18n script workflow.

---

### Task 1: Add backend integration-template DTOs and validation

**Files:**
- Modify: `dto/model_playground.go`
- Modify: `dto/model_playground_test.go`

- [ ] **Step 1: Add failing parse and validation tests**

Add deterministic tests covering a valid two-interface video template, duplicate keys, unsupported methods, invalid paths, unsupported variables, and oversized text:

```go
func TestParseModelEndpointConfigsIntegration(t *testing.T) {
	configs, err := ParseModelEndpointConfigs(`{
		"openai-video": {
			"path": "/v1/videos",
			"playground": {
				"capabilities": ["video.text_to_video"],
				"integration": {
					"overview": "Create then query the task.",
					"interfaces": [
						{"key":"create","title":"Create","method":"POST","path":"/v1/videos","curl_template":"curl '{{base_url}}/v1/videos'"},
						{"key":"query","title":"Query","method":"GET","path":"/v1/videos/{{task_id}}","curl_template":"curl '{{base_url}}/v1/videos/{{task_id}}'"}
					]
				}
			}
		}
	}`)
	require.NoError(t, err)
	require.NotNil(t, configs["openai-video"].Playground.Integration)
	assert.Len(t, configs["openai-video"].Playground.Integration.Interfaces, 2)
}
```

Use table cases expecting errors containing `duplicate interface key`, `unsupported method`, `path must start with /`, `unsupported template variable`, and `template text is too long`.

- [ ] **Step 2: Run the focused backend test and verify failure**

Run:

```bash
go test ./dto -run 'TestParseModelEndpointConfigsIntegration|TestValidatePlaygroundIntegration' -count=1
```

Expected: FAIL because integration DTOs and validation do not exist.

- [ ] **Step 3: Add the backend types**

Add these types and attach `Integration` to `ModelPlaygroundConfig`:

```go
type PlaygroundIntegrationInterface struct {
	Key                string   `json:"key"`
	Title              string   `json:"title"`
	Description        string   `json:"description,omitempty"`
	Method             string   `json:"method"`
	Path               string   `json:"path"`
	RequestDescription string   `json:"request_description,omitempty"`
	CurlTemplate       string   `json:"curl_template"`
	ResponseExample    string   `json:"response_example,omitempty"`
	Notes              []string `json:"notes,omitempty"`
}

type PlaygroundIntegrationConfig struct {
	Overview         string                           `json:"overview,omitempty"`
	DocumentationURL string                           `json:"documentation_url,omitempty"`
	Interfaces       []PlaygroundIntegrationInterface `json:"interfaces"`
	ResultNote       string                           `json:"result_note,omitempty"`
	CompleteExample  string                           `json:"complete_example,omitempty"`
}

type ModelPlaygroundConfig struct {
	Capabilities []string                     `json:"capabilities,omitempty"`
	Parameters   []PlaygroundParameter        `json:"parameters,omitempty"`
	Integration  *PlaygroundIntegrationConfig `json:"integration,omitempty"`
}
```

- [ ] **Step 4: Implement bounded validation**

Inside `validateModelPlaygroundConfig`, validate the optional integration configuration with these rules:

- Maximum 8 interfaces.
- Maximum 64 characters for key, 120 for title, 500 for descriptions/notes, 20,000 for curl/response/complete examples.
- Interface keys must be non-empty and unique.
- Methods are limited to `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.
- Paths must start with `/`.
- `documentation_url` is empty or an absolute HTTP/HTTPS URL.
- Template variables are limited to `base_url`, `api_key`, `model`, `group`, `prompt`, `parameters_json`, `request_json`, `request_curl`, `reference_url`, and `task_id`.

Use one compiled regexp to scan `{{variable}}` tokens and return descriptive errors from the existing parser.

- [ ] **Step 5: Run backend tests**

Run:

```bash
go test ./dto -run 'TestParseModelEndpointConfigs|TestValidatePlayground' -count=1
```

Expected: PASS.

---

### Task 2: Add frontend types, built-in templates, and fallback resolution

**Files:**
- Modify: `web/default/src/features/models/lib/model-capabilities.ts`
- Create: `web/default/src/features/models/lib/model-capabilities.test.ts`
- Modify: `web/default/src/features/playground/types.ts`

- [ ] **Step 1: Add failing frontend tests**

Test these contracts:

```ts
test('openai-video template defines create and query interfaces', () => {
  const integration = CAPABILITY_TEMPLATES['openai-video'].playground?.integration
  expect(integration?.interfaces.map((item) => item.key)).toEqual([
    'create',
    'query',
  ])
})

test('custom integration overrides the built-in template', () => {
  const definition = getEndpointDefinition(
    parseModelEndpointDefinitions(JSON.stringify({
      'openai-video': {
        playground: {
          integration: {
            overview: 'Custom',
            interfaces: [{
              key: 'create',
              title: 'Custom create',
              method: 'POST',
              path: '/v1/videos',
              curl_template: 'custom',
            }],
          },
        },
      },
    })),
    'openai-video'
  )
  expect(definition.playground?.integration?.overview).toBe('Custom')
})
```

- [ ] **Step 2: Run the focused test and verify failure**

Run:

```bash
cd web/default
bun test src/features/models/lib/model-capabilities.test.ts
```

Expected: FAIL because the integration types/templates do not exist.

- [ ] **Step 3: Add frontend integration types**

Define shared frontend types in `model-capabilities.ts` and mirror them in playground API types:

```ts
export interface PlaygroundIntegrationInterfaceDefinition {
  key: string
  title: string
  description?: string
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  path: string
  request_description?: string
  curl_template: string
  response_example?: string
  notes?: string[]
}

export interface PlaygroundIntegrationDefinition {
  overview?: string
  documentation_url?: string
  interfaces: PlaygroundIntegrationInterfaceDefinition[]
  result_note?: string
  complete_example?: string
}
```

Add `integration?: PlaygroundIntegrationDefinition` beside `capabilities` and `parameters`.

- [ ] **Step 4: Add built-in protocol templates**

Extend the existing templates without adding another registry. At minimum:

- `openai`: one Chat Completions interface with explanation and response example.
- `gemini`: one configured Gemini interface.
- `image-generation`: generation interface plus edit-oriented notes/placeholders.
- `openai-video`: exactly two interfaces, `create` and `query`, plus an optional result note and commented complete Bash example.

The video create template must contain `{{model}}`; the query template must contain `{{task_id}}`. The complete example must contain comments `# 1. Create video task` and `# 2. Query video task` and must never retry the creation request.

- [ ] **Step 5: Preserve integration when saving capabilities**

Update capability merge logic so saving parameters does not erase an existing integration:

```ts
playground: {
  capabilities,
  parameters: savedParameters,
  integration,
}
```

When applying a built-in template, copy its integration object rather than sharing mutable arrays.

- [ ] **Step 6: Run frontend model-template tests**

Run:

```bash
bun test src/features/models/lib/model-capabilities.test.ts
```

Expected: PASS.

---

### Task 3: Build the administrator integration-template editor

**Files:**
- Modify: `web/default/src/features/models/components/drawers/model-capabilities-drawer.tsx`
- Create: `web/default/src/features/models/components/drawers/model-integration-template-editor.tsx`

- [ ] **Step 1: Extract a focused editor component**

Create `ModelIntegrationTemplateEditor` with controlled props:

```ts
type Props = {
  value: PlaygroundIntegrationDefinition | undefined
  onChange: (value: PlaygroundIntegrationDefinition | undefined) => void
  previewVariables: PlaygroundIntegrationVariables
}
```

Render existing `SideDrawerSection`, `Input`, `Textarea`, `Select`, `Button`, and sortable up/down actions. Do not add dependencies.

- [ ] **Step 2: Add interface editing**

Each interface editor must support:

- key
- title
- description
- method
- path
- request description
- curl template
- response example
- newline-delimited notes
- move up, move down, delete

“Add interface” appends:

```ts
{
  key: `interface_${value.interfaces.length + 1}`,
  title: 'New interface',
  method: 'POST',
  path: '/',
  curl_template: "curl '{{base_url}}/'",
}
```

- [ ] **Step 3: Add template application and preview**

Provide:

- “Use built-in template” to deep-copy the current endpoint template.
- “Remove custom template” to restore fallback behavior.
- A plain-text preview resolved with `https://api.frimodel.com`, `$NEW_API_KEY`, the current model name, `default`, a sample prompt, `{}`, a sample reference URL, and `task_example`.

Preview code must use the same resolver added in Task 4, rendered in `<pre>` without `dangerouslySetInnerHTML`.

- [ ] **Step 4: Wire state into the existing drawer**

Add `integration` state, load it from current definition or template, retain it when switching parameter controls, and save it into the existing endpoint JSON. Keep the existing success toast and mutation path.

- [ ] **Step 5: Add client-side validation before save**

Reject duplicate keys, empty title/key/path/template, unsupported method, paths without `/`, and unresolved variables. Display the existing error toast with translated messages.

---

### Task 4: Replace the curl builder with a structured guide resolver

**Files:**
- Create: `web/default/src/features/playground/lib/curl/playground-integration-guide.ts`
- Create: `web/default/src/features/playground/lib/curl/playground-integration-guide.test.ts`
- Modify: `web/default/src/features/playground/lib/index.ts`
- Remove after migration: `web/default/src/features/playground/lib/curl/playground-curl.ts`
- Remove after migration: `web/default/src/features/playground/lib/curl/playground-curl.test.ts`

- [ ] **Step 1: Write failing resolver tests**

Cover:

```ts
test('video guide resolves exactly create and query interfaces', () => {
  const guide = resolvePlaygroundIntegrationGuide({
    apiBaseUrl: 'https://api.frimodel.com',
    definition: CAPABILITY_TEMPLATES['openai-video'].playground!.integration!,
    model: 'seedance2.0',
    group: 'Seedance2.0',
    prompt: 'A slow camera move',
    parameters: { seconds: 15, size: '1280x720' },
  })
  expect(guide.interfaces.map((item) => item.key)).toEqual(['create', 'query'])
  expect(guide.interfaces[0].curl).toContain('"model": "seedance2.0"')
  expect(guide.interfaces[1].curl).toContain('$TASK_ID')
})

test('unknown variables remain visible', () => {
  expect(resolveIntegrationTemplate('{{unknown}}', variables)).toBe('{{unknown}}')
})
```

Also test apostrophe-safe prompt insertion, empty optional values, result note, and explanatory comments in `completeExample`.

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
bun test src/features/playground/lib/curl/playground-integration-guide.test.ts
```

Expected: FAIL because the resolver does not exist.

- [ ] **Step 3: Implement typed resolution**

Export:

```ts
export type PlaygroundIntegrationVariables = {
  base_url: string
  api_key: string
  model: string
  group: string
  prompt: string
  parameters_json: string
  request_json: string
  request_curl: string
  reference_url: string
  task_id: string
}

export function resolveIntegrationTemplate(
  template: string,
  variables: PlaygroundIntegrationVariables
): string

export function resolvePlaygroundIntegrationGuide(
  options: ResolvePlaygroundIntegrationGuideOptions
): PlaygroundIntegrationGuide
```

Use a single `replace` call matching `{{name}}`. Replace known variables only. Use `$NEW_API_KEY` and `$TASK_ID` as display defaults. Serialize parameter examples with two-space JSON indentation. Keep all output plain strings.

- [ ] **Step 4: Run resolver tests**

Run:

```bash
bun test src/features/playground/lib/curl/playground-integration-guide.test.ts
```

Expected: PASS.

---

### Task 5: Build the shared integration-guide dialog

**Files:**
- Create: `web/default/src/features/playground/components/input/playground-api-integration-button.tsx`
- Create: `web/default/src/features/playground/components/input/playground-api-integration-dialog.tsx`
- Remove after migration: `web/default/src/features/playground/components/input/playground-copy-curl-button.tsx`

- [ ] **Step 1: Build the toolbar trigger**

Reuse `PromptInputButton`, the existing terminal icon, tooltip, and translated label `API integration`. The button opens the dialog and is disabled only when no model is selected.

- [ ] **Step 2: Render the guide overview**

The dialog header shows selected model and base URL. The body starts with overview/authentication text, not code.

- [ ] **Step 3: Render ordered interface sections**

For every interface render:

- Step number and title.
- Method badge and path.
- Description and request description.
- Scrollable code block.
- “Copy curl” action using `useCopyToClipboard`.
- Response example when present.
- Notes as a compact list.

Use semantic headings and `aria-labelledby`. Code blocks use `overflow-x-auto`, `whitespace-pre`, and a bounded height.

- [ ] **Step 4: Render complete example and result note**

Show the optional complete example in a collapsed section with “Copy complete example”. Show result/storage guidance after the primary interfaces. The optional content-download curl is visually subordinate and is not numbered as Interface 3.

- [ ] **Step 5: Add documentation link**

Render configured documentation URLs with `target='_blank'` and `rel='noreferrer'`. Do not render an invalid URL.

---

### Task 6: Integrate chat, image, and video state

**Files:**
- Modify: `web/default/src/features/playground/components/input/playground-input.tsx`
- Modify: `web/default/src/features/playground/components/input/playground-input-tools.tsx`
- Modify: `web/default/src/features/playground/components/media/playground-media-input.tsx`
- Modify: `web/default/src/features/playground/types.ts`

- [ ] **Step 1: Expose the selected endpoint integration definition**

Extend `ModelOption.endpoints[].playground` with `integration`. Resolve the currently selected endpoint definition using the same endpoint selection already used for parameters and capabilities.

- [ ] **Step 2: Build chat guide variables**

Pass selected model, group, current draft, enabled parameter example JSON, base URL, and the chat endpoint integration definition into the shared button.

- [ ] **Step 3: Build image/video guide variables**

Pass selected model, group, prompt, current parameter values, reference URL or local-file placeholder, and the selected endpoint integration definition. Do not derive interface count from the runtime payload.

- [ ] **Step 4: Preserve layout**

Place the API integration action where the curl icon currently exists. Do not add another toolbar row. Verify it does not move the send, parameter, upload, or clear-history controls.

---

### Task 7: Internationalization

**Files:**
- Modify through script: `web/default/src/i18n/locales/en.json`
- Modify through script: `web/default/src/i18n/locales/zh.json`
- Modify through script: `web/default/src/i18n/locales/zh-TW.json`
- Modify through script: `web/default/src/i18n/locales/fr.json`
- Modify through script: `web/default/src/i18n/locales/ja.json`
- Modify through script: `web/default/src/i18n/locales/ru.json`
- Modify through script: `web/default/src/i18n/locales/vi.json`
- Modify generated report: `web/default/src/i18n/locales/_reports/_sync-report.json`

- [ ] **Step 1: Add user-facing keys through the project script**

Add keys including:

- `API integration`
- `Interface {{number}}`
- `Request example`
- `Response example`
- `Copy curl`
- `Copy complete example`
- `Complete workflow`
- `Result handling`
- `Save the task ID returned by this interface`
- `Poll every 5–10 seconds until the task completes or fails`
- `Use built-in template`
- `Remove custom template`
- `API integration template`
- `Add interface`
- `Request description`
- `Documentation URL`
- `Preview`

Use a temporary `web/default/scripts/add-missing-keys.mjs`, run it, run `bun run i18n:sync`, then remove the temporary script with `apply_patch`.

- [ ] **Step 2: Verify translations**

Run:

```bash
bun run i18n:sync
```

Expected: no missing key introduced by this feature.

---

### Task 8: Full verification and production rollout

**Files:**
- All changed files above.

- [ ] **Step 1: Run focused tests**

```bash
go test ./dto -run 'TestParseModelEndpointConfigs|TestValidatePlayground' -count=1
cd web/default
bun test src/features/models/lib/model-capabilities.test.ts
bun test src/features/playground/lib/curl/playground-integration-guide.test.ts
```

Expected: PASS.

- [ ] **Step 2: Run frontend static validation**

```bash
bun run typecheck
bunx oxlint -c .oxlintrc.json <all changed TypeScript and TSX files>
bunx oxfmt --check <all changed TypeScript, TSX, JSON, and Markdown files>
bun run i18n:sync
bun run build
```

Expected: PASS with no changed-file errors.

- [ ] **Step 3: Run repository checks**

```bash
cd /Users/mini/develop/project/new-api
git diff --check
```

Expected: no whitespace errors.

- [ ] **Step 4: Perform local/production E2E**

Verify:

- Text shows one documented interface with explanation, curl, and response example.
- Image generation and editing show operation-appropriate documentation.
- Video shows exactly Interface 1 Create and Interface 2 Query.
- Selected model text is preserved exactly.
- Complete video example contains explanatory comments and both interface calls.
- Optional download guidance is not numbered as Interface 3.
- Administrator can apply, edit, preview, save, close, reopen, and recover the custom template.
- Keyboard focus remains inside the dialog.
- At 390×844 there is no page-level horizontal overflow.
- Desktop light and dark themes remain consistent with existing styles.

- [ ] **Step 5: Deploy only the production master after all checks pass**

Build a clean `linux/amd64` image containing only the intended changes, verify its archive checksum, back up the current master image, load the new image on `ubuntu@148.113.178.75`, and recreate only `/data/frimodel/master` service `new-api-master`. Do not recreate worker, CPA, or nginx services.

- [ ] **Step 6: Verify production health**

Confirm:

- `friday-new-api-master` is healthy with restart count 0.
- `https://platform.frimodel.com/api/status` returns 200.
- `https://platform.frimodel.com/playground` returns 200.
- Worker, both CPA containers, and nginx retain their pre-deploy image IDs and start times.
- Production E2E passes for the integration dialog and administrator template save/reload.

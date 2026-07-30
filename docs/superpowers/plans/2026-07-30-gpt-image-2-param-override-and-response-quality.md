# GPT-image-2 Parameter Override and Response Quality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate GPT-image-2 prompt parameter rendering and response-quality echo into the existing per-channel parameter override editor for image generation and editing.

**Architecture:** Extend operation-based parameter overrides with a request-only `append_template` mode and a response-only `set_from_request` mode. Request operations run through the existing generation and multipart-edit override points; OpenAI image responses run response operations before output. Keep legacy channel settings readable while migrating the default UI to one parameter-override preset.

**Tech Stack:** Go 1.22+, Gin, gjson/sjson, testify, React 19, TypeScript, React Hook Form, Zod, i18next, Bun

---

## File Structure

- `relay/common/override.go`: Parse operation phase, render request-body templates, and apply response operations from the original normalized request.
- `relay/common/override_test.go`: Protect template rendering, phase isolation, response copying, missing values, and channel isolation semantics.
- `relay/channel/openai/relay_image.go`: Patch non-streaming generation/edit responses before writing them to clients.
- `relay/channel/openai/image_stream_test.go`: Protect top-level response quality behavior without decoding large image payloads.
- `relay/image_handler.go`: Keep the legacy prompt setting as a compatibility fallback and preserve pass-through precedence.
- `web/default/src/features/channels/components/dialogs/param-override-editor-dialog.tsx`: Add the two operation modes and GPT-image-2 preset.
- `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`: Remove the dedicated GPT Image configuration block.
- `web/default/src/features/channels/lib/channel-form.ts`: Migrate legacy settings into `param_override` during hydration and stop serializing the old setting from the default UI.
- `web/default/src/features/channels/lib/channel-form.test.mjs`: Protect migration and serialization.
- `web/default/src/features/channels/types.ts`: Remove default-UI-only legacy form fields while retaining server payload compatibility where required.
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`: Replace dedicated setting copy with parameter override operation and preset copy.

### Task 1: Parameter override request and response operations

**Files:**
- Modify: `relay/common/override.go`
- Modify: `relay/common/override_test.go`

- [ ] **Step 1: Add failing request-template tests**

Add table tests using this operation:

```go
override := map[string]interface{}{
    "operations": []interface{}{
        map[string]interface{}{
            "phase": "request",
            "path":  "prompt",
            "mode":  "append_template",
            "value": "\n\nOutput image requirements: size=${body.size}; quality=${body.quality}.",
        },
    },
}
```

Assert complete rendering, one missing value rendering as `auto`, both missing values producing no change, duplicate suffix prevention, and malformed variables returning an error.

- [ ] **Step 2: Run the focused request tests**

Run: `go test ./relay/common -run 'TestApplyParamOverrideAppendTemplate' -count=1`

Expected: FAIL because `append_template` is unsupported.

- [ ] **Step 3: Implement request template rendering**

Add `Phase string` to `ParamOperation`, parse it with an empty value normalized to `request`, and add `append_template` to path-based modes. Resolve `${body.<gjson path>}` against the operation input, replace a missing scalar with `auto`, skip when every referenced field is absent, reject malformed or non-scalar variables, and avoid appending an identical suffix twice.

- [ ] **Step 4: Add failing response-operation tests**

Construct a `RelayInfo` whose `Request` is:

```go
&dto.ImageRequest{Model: "client-alias", Quality: "low"}
```

and whose `UpstreamModelName` is `gpt-image-2`. Assert a response operation with `phase: response`, `mode: set_from_request`, `from: quality`, and `path: quality` writes top-level `quality: low`, overwrites an upstream value, skips an absent source, and respects the model condition using the mapped upstream model.

- [ ] **Step 5: Implement response operation application**

Add:

```go
func ApplyResponseParamOverrideWithRelayInfo(responseBody []byte, info *RelayInfo) ([]byte, error)
```

Marshal `info.Request` through `common.Marshal`, replace its model with `info.UpstreamModelName` when set, evaluate only `phase: response` operations against that request JSON, and implement `set_from_request` with `sjson.SetBytes`. Existing request application must ignore response-phase operations; operations without a phase remain request operations.

- [ ] **Step 6: Run parameter override tests**

Run: `go test ./relay/common -run 'TestApplyParamOverrideAppendTemplate|TestApplyResponseParamOverride' -count=1`

Expected: PASS.

### Task 2: OpenAI image response integration

**Files:**
- Modify: `relay/channel/openai/relay_image.go`
- Modify: `relay/channel/openai/image_stream_test.go`

- [ ] **Step 1: Add failing handler tests**

Exercise `OpenaiImageHandler` with generation and editing relay modes, a large-looking `b64_json` payload, and the response operation. Assert the emitted body preserves all existing fields and contains top-level `quality` from the original request. Add cases for conflicting upstream quality, missing request quality, and a channel without the operation.

- [ ] **Step 2: Run the focused handler tests**

Run: `go test ./relay/channel/openai -run 'TestOpenaiImageHandler.*Quality' -count=1`

Expected: FAIL because the handler currently forwards the upstream body unchanged.

- [ ] **Step 3: Apply response overrides before output**

After validating the upstream error and before `service.IOCopyBytesGracefully`, call `relaycommon.ApplyResponseParamOverrideWithRelayInfo`. Convert an invalid channel response operation into `types.ErrorCodeChannelParamOverrideInvalid` with skip-retry semantics. Continue using byte-level response patching so `data[].b64_json` is not decoded and re-encoded.

- [ ] **Step 4: Run OpenAI image tests**

Run: `go test ./relay/channel/openai -run 'TestOpenaiImageHandler.*Quality|TestOpenaiImageHandler' -count=1`

Expected: PASS.

### Task 3: Default UI consolidation and legacy migration

**Files:**
- Modify: `web/default/src/features/channels/components/dialogs/param-override-editor-dialog.tsx`
- Modify: `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- Modify: `web/default/src/features/channels/lib/channel-form.ts`
- Modify: `web/default/src/features/channels/lib/channel-form.test.mjs`
- Modify: `web/default/src/features/channels/types.ts`

- [ ] **Step 1: Add failing form migration tests**

Given a channel whose setting contains enabled `image_prompt_parameter_append`, assert hydration merges an equivalent `append_template` operation into `param_override` once. Assert payload serialization omits `image_prompt_parameter_append` and preserves both the migrated operation and unrelated existing operations.

- [ ] **Step 2: Run the form tests**

Run: `cd web/default && bun test src/features/channels/lib/channel-form.test.mjs`

Expected: FAIL because the old fields are still hydrated and serialized separately.

- [ ] **Step 3: Implement migration and remove dedicated fields**

Add a focused migration function that parses operation JSON, detects an equivalent GPT-image-2 template operation, and appends one only when needed. Use it while hydrating an existing channel. Remove the dedicated form schema/default/payload fields and remove the dedicated drawer section without altering unrelated channel settings.

- [ ] **Step 4: Add operation modes and preset**

Extend the visual editor operation type with `phase`. Add `append_template` with path/value fields and `set_from_request` with path/from fields. Normalize `set_from_request` to response phase and all other omitted phases to request. Add one preset containing the two approved GPT-image-2 operations.

- [ ] **Step 5: Run form tests**

Run: `cd web/default && bun test src/features/channels/lib/channel-form.test.mjs`

Expected: PASS.

### Task 4: Internationalization and verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Add translations**

Add translations for `Append Request Template`, `Set Response from Request`, their descriptions, and `GPT-image-2 Size/Quality Compatibility`. Remove only translations that became unused with the dedicated configuration block.

- [ ] **Step 2: Format changed source files**

Run `gofmt` on changed Go files and the project frontend formatter on changed TypeScript files.

- [ ] **Step 3: Run backend regression tests**

Run:

```bash
go test ./relay/common ./relay/channel/openai ./relay ./dto ./model -count=1
```

Expected: PASS.

- [ ] **Step 4: Run frontend checks**

Run from `web/default`:

```bash
bun test src/features/channels/lib/channel-form.test.mjs
bun run typecheck
bun run lint
bun run build
```

Expected: PASS, with no lint errors in changed files and a complete production bundle.

- [ ] **Step 5: Inspect the final diff**

Run `git diff --check` and inspect only task-related files. Confirm unrelated dirty-worktree files are neither staged nor modified by formatting.

### Task 5: Commit and deploy the complete source tree to master

**Files:**
- Modify: task-related source, tests, design, and plan files only

- [ ] **Step 1: Commit the implementation**

Stage only the task-related files and commit with:

```bash
git commit -m "feat: integrate gpt image compatibility overrides"
```

- [ ] **Step 2: Inspect the live master deployment**

SSH to `ubuntu@148.113.178.75`, identify the master compose directory, current image ID, container name, health state, and available disk space without changing live state.

- [ ] **Step 3: Build the complete application image**

Build from the repository root so the backend and full default frontend are included. Do not copy only changed files or build a partial artifact. Tag the resulting image with a unique timestamped deployment tag.

- [ ] **Step 4: Transfer and load the image**

Save the complete image, transfer it to the server, load it, and verify the loaded image digest matches the local artifact.

- [ ] **Step 5: Prepare rollback and switch master**

Tag the currently running master image with a timestamped rollback tag. Update only the master deployment to the newly loaded image and recreate the master container using its existing compose configuration.

- [ ] **Step 6: Verify or immediately revert**

Check container health, restart count, recent error logs, and the live health/status endpoint immediately after the switch. If any check fails, restore the saved master image tag and recreate the container before doing further diagnosis.

- [ ] **Step 7: Report deployment evidence**

Report the implementation commit, deployed image digest/tag, master container health and restart count, and rollback tag. Do not update the worker unless the user separately requests it.

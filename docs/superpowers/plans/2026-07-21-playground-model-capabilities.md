# Playground Model Capabilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing Playground with administrator-configurable image and video capabilities while preserving the current chat flow, components, storage, and database schema.

**Architecture:** Store optional Playground capability metadata inside the existing `models.endpoints` JSON objects. Add a third section to the existing models page for editing that metadata, expose detailed model options through the existing user-model endpoint, and add session-authenticated `/pg` wrappers around the existing image and video relay handlers. The current chat components and storage remain unchanged.

**Tech Stack:** Go, Gin, GORM, React 19, TypeScript, TanStack Query/Router, React Hook Form, Zod, Base UI, Tailwind CSS, Vitest, Bun.

---

### Task 1: Define and parse endpoint capability metadata

**Files:**
- Create: `dto/model_playground.go`
- Create: `dto/model_playground_test.go`
- Modify: `common/endpoint_defaults.go`

- [ ] Add typed JSON structures for endpoint path, method, Playground capabilities, and `string`/`number`/`boolean`/`enum` parameters.
- [ ] Parse both legacy string endpoints and object endpoints without changing their existing routing meaning.
- [ ] Reject duplicate parameter keys, unsupported parameter types, invalid enum defaults, and invalid numeric ranges.
- [ ] Add the missing default `openai-video` endpoint mapping to `/v1/videos`.
- [ ] Run `go test ./dto ./common` and verify the new deterministic table tests pass.

### Task 2: Return detailed models without breaking the existing API

**Files:**
- Modify: `controller/user.go`
- Modify: `controller/model_list_test.go`
- Modify: `model/model_meta.go`

- [ ] Add a batch model-metadata lookup that respects exact, prefix, suffix, and contains model rules.
- [ ] Keep `GET /api/user/models?group=...` returning `string[]` by default.
- [ ] When `details=true`, return model name, raw endpoint configuration, derived endpoint types, and parsed Playground capabilities.
- [ ] Add tests proving group filtering, legacy response compatibility, and detailed capability output.
- [ ] Run `go test ./controller -run 'TestGetUserModels'` and verify all cases pass.

### Task 3: Add the model capability administration section

**Files:**
- Modify: `web/default/src/features/models/section-registry.tsx`
- Modify: `web/default/src/features/models/index.tsx`
- Modify: `web/default/src/features/models/types.ts`
- Create: `web/default/src/features/models/lib/model-capabilities.ts`
- Create: `web/default/src/features/models/lib/model-capabilities.test.ts`
- Create: `web/default/src/features/models/components/model-capabilities-table.tsx`
- Create: `web/default/src/features/models/components/drawers/model-capabilities-drawer.tsx`

- [ ] Add `capabilities` as a third existing models-page section and render it with `SectionPageLayout` and the current Tabs.
- [ ] Reuse `getModels`, `updateModel`, Table, Drawer, Input, Select, Switch, Button, Badge, and JsonEditor components.
- [ ] Parse legacy endpoint arrays/objects and serialize changes back into the existing `endpoints` string only.
- [ ] Provide built-in templates for OpenAI Image, Gemini Image, OpenAI Video, Seedance, and Grok Video parameters.
- [ ] Let administrators enable parameters, edit labels/defaults/options/ranges, and add controlled custom parameters.
- [ ] Keep the existing raw endpoint editor available in the model metadata drawer.
- [ ] Run the focused Vitest file and verify parse/serialize round trips preserve unrelated endpoint fields.

### Task 4: Add session-authenticated image and video Playground routes

**Files:**
- Modify: `controller/playground.go`
- Create: `controller/playground_test.go`
- Modify: `router/relay-router.go`
- Modify: `router/video-router.go`

- [ ] Refactor the existing temporary Playground token setup into reusable controller setup used by chat, image, and task handlers.
- [ ] Add `/pg/images/generations` and `/pg/images/edits` wrappers around `RelayFormatOpenAIImage`.
- [ ] Add `/pg/videos`, `/pg/videos/:task_id`, and `/pg/videos/:task_id/content` wrappers around existing task and video-content handlers.
- [ ] Validate configured Playground parameters before relay and return HTTP 400 for values not exposed by the model profile.
- [ ] Preserve existing `/v1` API behavior; capability limits apply only to `/pg` routes.
- [ ] Run the focused controller and router tests.

### Task 5: Extend model loading and mode selection without replacing chat

**Files:**
- Modify: `web/default/src/features/playground/api.ts`
- Modify: `web/default/src/features/playground/types.ts`
- Modify: `web/default/src/features/playground/constants.ts`
- Modify: `web/default/src/features/playground/hooks/use-playground-options.ts`
- Modify: `web/default/src/features/playground/index.tsx`
- Create: `web/default/src/features/playground/lib/capabilities/model-capability-utils.ts`
- Create: `web/default/src/features/playground/lib/capabilities/model-capability-utils.test.ts`

- [ ] Fetch `/api/user/models?details=true` and normalize legacy responses defensively.
- [ ] Add the mode type `chat | image | video` and filter model options by capability.
- [ ] Add an existing Tabs control above the Playground content.
- [ ] Leave `PlaygroundChat`, `usePlaygroundConversation`, `useChatHandler`, and all existing storage keys unchanged for chat mode.
- [ ] Persist only the selected mode in a new independent storage key so old messages and configuration remain readable.
- [ ] Test capability filtering and fallback behavior.

### Task 6: Add dynamic image generation and editing UI

**Files:**
- Create: `web/default/src/features/playground/components/media/playground-parameter-fields.tsx`
- Create: `web/default/src/features/playground/components/media/playground-image.tsx`
- Create: `web/default/src/features/playground/hooks/use-image-generation.ts`
- Create: `web/default/src/features/playground/lib/media/media-payload-builder.ts`
- Create: `web/default/src/features/playground/lib/media/media-payload-builder.test.ts`

- [ ] Render configured enum, string, number, and boolean parameters using existing form controls.
- [ ] Support JSON image generation and multipart image editing with existing API and file-input conventions.
- [ ] Display returned URL or Base64 images in a responsive result area with existing Card, Button, Skeleton, and error styles.
- [ ] Preserve generated results in the existing message storage format by storing assistant image Markdown content instead of creating a second history database.
- [ ] Add exact payload-builder tests for GPT Image and Gemini Image profiles.

### Task 7: Add asynchronous video task UI

**Files:**
- Create: `web/default/src/features/playground/components/media/playground-video.tsx`
- Create: `web/default/src/features/playground/hooks/use-video-generation.ts`
- Modify: `web/default/src/features/playground/lib/media/media-payload-builder.ts`
- Modify: `web/default/src/features/playground/lib/media/media-payload-builder.test.ts`

- [ ] Build configured text-to-video and image-to-video request bodies.
- [ ] Poll existing task statuses with bounded intervals and stop on `completed` or `failed`.
- [ ] Reuse existing progress, Card, Button, Badge, and error components for queued/in-progress/completed/failed states.
- [ ] Use the authenticated `/pg/videos/{task_id}/content` URL for playback and download.
- [ ] Store task/result summaries in the existing Playground message storage shape.

### Task 8: Complete translations and verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] Add every new user-facing key to all supported locales using the project i18n workflow.
- [ ] Run `bun run i18n:sync` from `web/default` and review only relevant locale changes.
- [ ] Run focused Vitest tests, `bun run typecheck`, lint for touched files, and `bun run build`.
- [ ] Run focused Go tests followed by `go test ./controller ./dto ./common ./router`.
- [ ] Confirm `git diff --check` passes and inspect the final diff for unrelated changes.
- [ ] Do not run browser or end-to-end tests unless the user explicitly requests them.

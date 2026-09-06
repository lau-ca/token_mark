# Playground Power-User UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `/playground` into a model-first workspace with a consistent prompt, parameter, send, and result workflow for chat, image, and video users.

**Architecture:** Keep the existing mode-specific state, capability metadata, request functions, and message storage. Move mode and model context into a shared workspace header, enhance the existing combined selector for power users, and reshape media generation around the existing prompt-input visual language without changing backend contracts.

**Tech Stack:** React 19, TypeScript, TanStack Query, Base UI/shadcn components, Tailwind CSS, i18next, Bun.

---

### Task 1: Shared Playground Context Bar

**Files:**
- Create: `web/default/src/features/playground/components/playground-context-bar.tsx`
- Modify: `web/default/src/features/playground/index.tsx`

- [ ] **Step 1: Create the context-bar component**

Create a component that composes the existing `Tabs`, `TabsList`, `TabsTrigger`, and `ModelGroupSelector`. It receives the active mode, compatible models, groups, loading state, and selection callbacks. The model trigger uses a wider class so the current model remains readable.

```tsx
<div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
  <Tabs value={props.mode} onValueChange={props.onModeChange}>
    <TabsList>...</TabsList>
  </Tabs>
  <ModelGroupSelector className='h-9 w-full max-w-none md:w-[22rem]' ... />
</div>
```

- [ ] **Step 2: Replace the standalone tabs in `Playground`**

Render `PlaygroundContextBar` at the top and remove model selection props from chat/media inputs. Keep `mode_selections` as the authoritative per-mode selection.

- [ ] **Step 3: Run targeted type checking**

Run: `cd web/default && bun run typecheck`

Expected: no TypeScript errors.

### Task 2: Power-User Model Selector

**Files:**
- Modify: `web/default/src/components/model-group-selector.tsx`
- Modify: `web/default/src/components/model-group-selector-layout.ts`

- [ ] **Step 1: Make the trigger suitable as a primary control**

Allow the caller-provided class to control width, keep the full model name visible, and show the group as secondary metadata. Preserve the compact behavior for other consumers.

- [ ] **Step 2: Improve selection information architecture**

Keep the existing group column and searchable model list, but add a clear heading for the selected group, model count, and an explicit empty-state action hint. Use `Badge`, `Separator`, and existing command primitives instead of custom status pills or raw divider markup.

- [ ] **Step 3: Improve keyboard and mobile behavior**

Autofocus model search when opened, retain arrow/Enter selection through `Command`, and keep the current `Drawer` composition on mobile with a visible `DrawerTitle`.

- [ ] **Step 4: Run lint for the selector files**

Run: `cd web/default && bunx oxlint src/components/model-group-selector.tsx src/components/model-group-selector-layout.ts`

Expected: no lint errors.

### Task 3: Unified Chat and Media Composer

**Files:**
- Modify: `web/default/src/features/playground/components/input/playground-input.tsx`
- Modify: `web/default/src/features/playground/components/input/playground-input-controls.tsx`
- Modify: `web/default/src/features/playground/components/media/playground-media-input.tsx`
- Modify: `web/default/src/features/playground/components/media/playground-parameter-fields.tsx`

- [ ] **Step 1: Remove duplicate model selection from composers**

The shared context bar owns model selection. Chat controls retain tools and send/stop only. Media input no longer renders `ModelGroupSelector` inside its card.

- [ ] **Step 2: Reshape media input around the prompt**

Use the same rounded, bordered, focused input surface as `PromptInput`: prompt textarea first, compact parameter fields above or in a secondary section, reference input in the tool area, and generate action in the footer.

- [ ] **Step 3: Use field primitives for dynamic parameters**

Render each admin-defined parameter with `Field`, `FieldLabel`, `FieldDescription`, `Select`, `Switch`, or `Input`. Mark required fields and invalid empty values without changing the parameter schema.

- [ ] **Step 4: Preserve request behavior**

Keep `handleSubmit`, Gemini image conversion, multipart image editing, video polling, abort behavior, and message updates unchanged. Only move presentation and add derived validation text.

- [ ] **Step 5: Run typecheck and targeted lint**

Run: `cd web/default && bun run typecheck`

Run: `cd web/default && bunx oxlint src/features/playground/components/input/playground-input.tsx src/features/playground/components/input/playground-input-controls.tsx src/features/playground/components/media/playground-media-input.tsx src/features/playground/components/media/playground-parameter-fields.tsx`

Expected: both commands pass.

### Task 4: Result Context and Responsive Polish

**Files:**
- Modify: `web/default/src/features/playground/types.ts`
- Modify: `web/default/src/features/playground/components/chat/playground-chat.tsx`
- Modify: `web/default/src/features/playground/components/message/playground-message-content.tsx`
- Modify: `web/default/src/features/playground/components/chat/playground-empty-state.tsx`

- [ ] **Step 1: Add optional request context to messages**

Add an optional UI-only request context containing model, group, and parameter summary. It is persisted through the existing browser message storage because it remains part of `Message`.

```ts
requestContext?: {
  model: string
  group: string
  parameters?: Record<string, string | number | boolean>
}
```

- [ ] **Step 2: Attach context when requests are created**

Chat and media user/assistant message pairs receive the current model/group; media assistant messages also receive submitted parameter values. No API payload changes are required.

- [ ] **Step 3: Render a compact result summary**

Use `Badge` components near assistant results to show the model and selected key parameters. Do not show internal request paths or empty values.

- [ ] **Step 4: Tighten empty-state copy and spacing**

Explain that the user should choose a model in the top context bar and then enter a prompt. Keep the existing icon, button, and typography language.

### Task 5: Verification

**Files:**
- Modify only if verification reveals issues in files above.

- [ ] **Step 1: Check formatting and whitespace**

Run: `git diff --check`

Expected: no output.

- [ ] **Step 2: Run frontend typecheck**

Run: `cd web/default && bun run typecheck`

Expected: success.

- [ ] **Step 3: Run targeted lint**

Run oxlint against every changed TS/TSX file.

Expected: no lint errors.

- [ ] **Step 4: Build production frontend**

Run: `cd web/default && bun run build`

Expected: production build succeeds.

- [ ] **Step 5: Verify rendered behavior**

Confirm desktop and mobile layouts, keyboard model search, independent selections for all three modes, dynamic parameter changes after model switches, send/generate disabled states, and result context rendering.

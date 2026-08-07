# Model Capability Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep saved model capability configuration independent from channel lifecycle while allowing channels to control only runtime availability and Playground visibility.

**Architecture:** Build the capability administration catalog from the union of exact `models` records and runtime Pricing models. Preserve `models.endpoints` as the sole persistent capability source, add an availability flag derived from Pricing, and invalidate all dependent React Query caches after channel or capability mutations.

**Tech Stack:** Go, Gin, GORM, React 19, TypeScript, TanStack Query, Vitest, Bun.

---

### Task 1: Protect capability catalog persistence in the backend

**Files:**
- Modify: `model/model_meta.go`
- Modify: `controller/model_meta.go`
- Modify: `controller/model_capabilities_test.go`

- [ ] **Step 1: Write failing catalog tests**

Add tests named `TestBuildModelCapabilityCatalogKeepsConfiguredUnavailableModels`, `TestBuildModelCapabilityCatalogMergesConfiguredAndRuntimeModels`, and `TestBuildModelCapabilityCatalogDoesNotAttachRuleMetadata`. The first expects an exact model with saved `Endpoints` to remain with `Available == false` when Pricing is empty. The second expects configured-only and runtime-only models exactly once. The third preserves the exact-versus-rule editing boundary.

- [ ] **Step 2: Run the focused test and verify failure**

Run `go test ./controller -run 'TestBuildModelCapabilityCatalog' -count=1`. Expect the new tests to fail because the current builder accepts only Pricing models and has no availability field. If unrelated controller package compile failures prevent execution, record the exact output.

- [ ] **Step 3: Add an exact metadata query**

Add `GetExactModelMetadata()` in `model/model_meta.go`. It must query `name_rule = NameRuleExact`, order by `model_name ASC`, and return non-deleted GORM records.

- [ ] **Step 4: Build the union catalog**

Add `Available bool \`json:"available"\`` to the catalog item. Build a Pricing lookup by model name, append exact metadata models, then append runtime-only Pricing models. Merge stored endpoint names into `SupportedEndpointTypes` without writing them back. Mark `Available` true only when Pricing contains the model.

- [ ] **Step 5: Run focused backend validation**

Run the model pricing endpoint tests and `go test ./controller -run 'TestBuildModelCapabilityCatalog' -count=1`.

### Task 2: Show persistent configuration and runtime availability separately

**Files:**
- Modify: `web/src/features/models/types.ts`
- Modify: `web/src/features/models/components/model-capabilities-table.tsx`
- Create: `web/src/features/models/components/__tests__/model-capabilities-table.test.tsx`

- [ ] **Step 1: Write the failing component test**

Mock a catalog item containing saved `playground.capabilities` and `available: false`. Assert that the configured capability badge, Configure button, and unavailable-channel badge remain visible.

- [ ] **Step 2: Run the test and verify failure**

Run `cd web && bunx vitest run src/features/models/components/__tests__/model-capabilities-table.test.tsx` and expect failure because availability is not represented.

- [ ] **Step 3: Add availability rendering**

Extend `ModelCapabilityCatalogItem` with `available: boolean`. Keep capability badges driven solely by `metadata.endpoints`. Render `t('No available channel')` when unavailable and keep Configure enabled.

- [ ] **Step 4: Run the focused test**

Run the same Vitest command and expect PASS.

### Task 3: Invalidate dependent frontend caches

**Files:**
- Create: `web/src/features/models/lib/model-capability-query-invalidation.ts`
- Create: `web/src/features/models/lib/__tests__/model-capability-query-invalidation.test.ts`
- Modify: `web/src/features/models/lib/index.ts`
- Modify: `web/src/features/models/components/model-capabilities-table.tsx`
- Modify: `web/src/features/channels/lib/channel-actions.ts`
- Modify: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`

- [ ] **Step 1: Write failing invalidation tests**

Test one shared function that invalidates `modelsQueryKeys.capabilities()`, `['playground-models']`, and `['playground-model-catalog']` using prefix matching.

- [ ] **Step 2: Run the test and verify failure**

Run `cd web && bunx vitest run src/features/models/lib/__tests__/model-capability-query-invalidation.test.ts` and expect failure because the helper does not exist.

- [ ] **Step 3: Implement and reuse the helper**

Implement `invalidateModelCapabilityQueries(queryClient)` with `Promise.all` over the three query prefixes. Call it after capability save, channel enable/disable/delete, channel field changes, and full channel drawer save while preserving existing channel-list invalidation.

- [ ] **Step 4: Run focused frontend tests**

Run both new Vitest files and expect PASS.

### Task 4: Complete translations and verification

**Files:**
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`

- [ ] **Step 1: Add translations**

Add the English key `No available channel` to every supported locale using the project i18n workflow.

- [ ] **Step 2: Run frontend quality checks**

Run `bun run i18n:sync`, both focused Vitest files, `bun run typecheck`, lint for touched files, and `bun run build` from `web/`.

- [ ] **Step 3: Run backend and repository checks**

Run focused model tests, focused controller tests, and `git diff --check`. Inspect the final changed-file list to ensure unrelated dirty-tree files are not included in the implementation artifact.

### Task 5: Build and deploy only the master node

**Files:**
- No committed deployment file changes.
- Artifact: `outputs/new-api-model-capability-persistence-<timestamp>-amd64.tar.gz`

- [ ] **Step 1: Inspect the remote master without changing it**

Check `ubuntu@148.113.178.75` for `/root/gateway/master/docker-compose.yml`, `new-api-master`, the configured image reference, health, architecture, and local `/api/status`.

- [ ] **Step 2: Build an isolated linux/amd64 image**

Create a temporary clean build context from `HEAD`, overlay only this task's implementation files, build the exact master image reference, save it as a compressed tar, and calculate SHA-256. Do not include unrelated dirty-tree files.

- [ ] **Step 3: Upload and prepare rollback**

Upload the artifact. Tag the currently running image with a timestamped rollback tag and record the exact restore command before loading the new image.

- [ ] **Step 4: Load and recreate only master**

Load the image and run `sudo docker compose -f /root/gateway/master/docker-compose.yml up -d --force-recreate new-api-master`. Do not recreate worker services.

- [ ] **Step 5: Verify deployment**

Confirm the container uses the new image ID, is healthy, and returns HTTP 200 from server-local and public `/api/status`. If health fails, restore the timestamped image and recreate only `new-api-master`.

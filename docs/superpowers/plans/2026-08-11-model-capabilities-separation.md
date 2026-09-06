# Model Capabilities Separation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store Playground model capabilities independently from model metadata so capability changes never create metadata rows.

**Architecture:** Add a cross-database `model_capabilities` table with one JSON `TEXT` document per model. Expose dedicated capability read/write APIs, migrate legacy `models.endpoints.playground` data idempotently, and route Playground reads through the new model layer.

**Tech Stack:** Go, Gin, GORM v2, SQLite/MySQL/PostgreSQL, React 19, TypeScript, React Query, Vitest.

---

### Task 1: Persistence and migration

**Files:**
- Create: `model/model_capability.go`
- Create: `model/model_capability_test.go`
- Modify: `model/main.go`

- [ ] Add the `ModelCapability` model and JSON validation helpers.
- [ ] Add lookup and upsert functions keyed by `model_name`.
- [ ] Add an idempotent migration that extracts legacy endpoint `playground` objects while preserving all other endpoint fields.
- [ ] Register the table and migration in normal and fast database migration paths.
- [ ] Run `go test ./model -run 'ModelCapability|LegacyModelCapabilities'`.

### Task 2: Capability APIs and catalog

**Files:**
- Modify: `controller/model_meta.go`
- Modify: `controller/model_capabilities_test.go`
- Modify: `router/api-router.go`

- [ ] Build the catalog from runtime pricing plus persisted capability rows.
- [ ] Add a dedicated upsert endpoint that validates and saves the JSON document without writing `models`.
- [ ] Update catalog regression tests for configured, unavailable, and runtime-only models.
- [ ] Run `go test ./controller -run ModelCapability`.

### Task 3: Playground reads

**Files:**
- Modify: `controller/playground.go`
- Modify: `controller/playground_test.go`
- Modify: `controller/user.go`

- [ ] Resolve capabilities from `model_capabilities.config` for request validation.
- [ ] Populate detailed user model options from capability configuration while retaining runtime endpoint defaults.
- [ ] Add regression coverage for the independent read path.
- [ ] Run the affected controller tests.

### Task 4: Frontend save path

**Files:**
- Modify: `web/src/features/models/api.ts`
- Modify: `web/src/features/models/types.ts`
- Modify: `web/src/features/models/components/model-capabilities-table.tsx`
- Modify: `web/src/features/models/components/drawers/model-capabilities-drawer.tsx`
- Modify: `web/src/features/models/components/__tests__/model-capabilities-table.test.tsx`

- [ ] Replace metadata-based capability state with the catalog `config` field.
- [ ] Save through the dedicated capability endpoint.
- [ ] Prove saving a runtime-only model does not call model creation or update APIs.
- [ ] Run the affected Vitest file, TypeScript typecheck, and lint for modified files.

### Task 5: Verification

- [ ] Run targeted Go tests for model and controller packages.
- [ ] Run the affected frontend tests and `bun run typecheck`.
- [ ] Run `git diff --check` and inspect the final diff for unrelated changes.

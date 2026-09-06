# Playground Model Square Capabilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Model Square the authoritative capability-admin catalog and show only explicitly configured text, image, or video models in Playground.

**Architecture:** Add an admin-only catalog endpoint backed by `model.GetPricing()` and exact model metadata. Reuse `models.endpoints` for persistence, create exact metadata on first save when needed, and require explicit Playground capabilities in both frontend filtering and backend `/pg` validation.

**Tech Stack:** Go, Gin, GORM, React 19, TypeScript, TanStack Query, Base UI, i18next, Bun.

---

### Task 1: Add the Model Square capability catalog

**Files:**
- Modify: `controller/model_meta.go`
- Modify: `router/api-router.go`
- Test: `controller/model_list_test.go`

- [ ] Add an admin response type containing `model_name`, `supported_endpoint_types`, and optional exact model metadata.
- [ ] Build the response from `model.GetPricing()` and `model.GetModelMetadataByNames`, accepting metadata only when `name_rule == model.NameRuleExact` and `model_name` exactly matches.
- [ ] Register `GET /api/models/capabilities` before `GET /api/models/:id`.
- [ ] Add a deterministic controller/model test proving the catalog excludes metadata-only models and does not attach a rule record as exact metadata.
- [ ] Run `go test ./controller ./model` and expect both packages to pass.

### Task 2: Replace the admin capability page data source

**Files:**
- Modify: `web/default/src/features/models/api.ts`
- Modify: `web/default/src/features/models/types.ts`
- Modify: `web/default/src/features/models/components/model-capabilities-table.tsx`
- Modify: `web/default/src/features/models/components/drawers/model-capabilities-drawer.tsx`

- [ ] Add typed API support for `GET /api/models/capabilities`.
- [ ] Render and filter catalog items returned in Model Square order.
- [ ] Pass exact metadata into the existing drawer when available.
- [ ] On save, call `updateModel` for an existing exact record or `createModel` with `model_name`, `name_rule: 0`, `status: 1`, `sync_official: 1`, and serialized endpoints for a missing record.
- [ ] Invalidate the capability catalog query after save.
- [ ] Run targeted `oxlint` on the modified model feature files and expect no errors.

### Task 3: Require explicit capability configuration in Playground

**Files:**
- Modify: `web/default/src/features/playground/lib/capabilities/model-capability-utils.ts`
- Modify: `controller/playground.go`
- Test: `controller/playground_test.go`

- [ ] Remove endpoint-name fallback capabilities from user-facing model filtering; only `endpoint.playground.capabilities` makes a model selectable.
- [ ] Add backend validation that finds an explicitly configured capability across the model endpoints.
- [ ] Treat ordinary `/pg/chat/completions` requests as `chat`, while retaining Gemini image detection and image/video route-specific capability validation.
- [ ] Reject models with only inferred endpoint support and accept models with explicit matching capabilities.
- [ ] Run `go test ./dto ./controller ./middleware ./router ./relay` and expect all packages to pass.

### Task 4: Verify frontend behavior and translations

**Files:**
- Modify only when new user-facing strings are required: `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

- [ ] Run `bun run typecheck` from `web/default` and expect success.
- [ ] Run targeted `bunx oxlint` for all modified TypeScript and TSX files and expect zero errors.
- [ ] Run `bun run build` from `web/default` and expect success.
- [ ] Run `git diff --check` and expect no whitespace errors.

### Task 5: Deploy worker then master

**Files:**
- No repository file changes.

- [ ] Build a clean linux/amd64 Docker image containing only the Playground-related working-tree changes.
- [ ] Upload the image tar to `ubuntu@148.113.178.75` and preserve the current image under a new rollback tag.
- [ ] Recreate `friday-new-api-worker`, verify health, `/api/status`, and unauthenticated `/pg` responses.
- [ ] Recreate `friday-new-api-master`, verify health and both public status endpoints.
- [ ] Confirm Nginx, CPA, Redis, PostgreSQL, and Mago containers were not recreated.


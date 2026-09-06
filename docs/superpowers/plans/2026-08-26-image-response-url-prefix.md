# Image Response URL Prefix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-channel trusted image-response URL prefix and reject mismatched upstream URLs with a fixed 502 error.

**Architecture:** Persist the prefix in `ChannelOtherSettings`, expose it through the existing channel form, and centralize response validation in the OpenAI image relay. Each response path calls the same validator before writing client output.

**Tech Stack:** Go, Gin, relaykit DTOs, React 19, TypeScript, Zod, i18next, Bun, testify.

---

### Task 1: Backend setting and validation

**Files:**
- Modify: `relaykit/dto/channel_settings.go`
- Modify: `model/channel.go`
- Test: `model/channel_settings_test.go`

- [ ] Add `ImageResponseURLPrefix string` with JSON key `image_response_url_prefix`.
- [ ] Validate that only the trimmed value is used and preserve an empty value as unrestricted.
- [ ] Run `cd relaykit && GOWORK=off go test ./dto` and `go test ./model`.

### Task 2: Image response enforcement

**Files:**
- Modify: `relay/channel/openai/relay_image.go`
- Test: `relay/channel/openai/image_stream_test.go`
- Test: `relay/channel/openai/image_edit_test.go`

- [ ] Add table tests for hidden, unrestricted, matching, and mismatching URLs.
- [ ] Return `types.NewOpenAIError(errors.New("openai error."), types.ErrorCodeBadResponse, http.StatusBadGateway)` on mismatch.
- [ ] Use the shared check before output in JSON, JSON-to-SSE, and native SSE paths.
- [ ] Run `go test ./relay/channel/openai -count=1`.

### Task 3: Channel form and translations

**Files:**
- Modify: `web/src/features/channels/lib/channel-form.ts`
- Modify: `web/src/features/channels/types.ts`
- Modify: `web/src/features/channels/lib/channel-form-errors.ts`
- Modify: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- Modify through script: `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

- [ ] Add the optional string field, default, load/save transforms, advanced-setting detection, and error routing.
- [ ] Render one input only when the hide switch is disabled.
- [ ] Add translations through `web/scripts/add-missing-keys.mjs`, run it, remove it, then run `bun run i18n:sync`.
- [ ] Run targeted oxlint, TypeScript checking, and the production frontend build.

### Task 4: Review and master-only deployment

**Files:**
- Review only the files listed above plus deployment metadata.

- [ ] Inspect the final diff and run focused backend/frontend checks.
- [ ] Build a clean linux/amd64 image containing the intended source state without unrelated artifacts.
- [ ] On `ubuntu@148.113.178.75`, back up the master compose/image state and load the new image.
- [ ] Recreate only `friday-new-api-master`; do not restart worker, CPA, Redis, PostgreSQL, or nginx.
- [ ] Verify the master container image, health, and `/api/status` response.

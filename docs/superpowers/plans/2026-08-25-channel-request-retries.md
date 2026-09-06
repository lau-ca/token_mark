# Channel Request Retries Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow administrators to configure zero to three same-channel retries for all models on an individual channel.

**Architecture:** Store the retry count in the existing channel `setting` JSON so no database migration is required. The normal relay path retries the already-selected channel before falling back to the existing global cross-channel retry loop, and only retries retryable failures before any response bytes are written. Task and realtime relay paths remain unchanged.

**Tech Stack:** Go, Gin, relaykit DTOs, React 19, TypeScript, React Hook Form, Zod, i18next, Vitest.

---

### Task 1: Channel setting contract

**Files:**
- Modify: `relaykit/dto/channel_settings.go`
- Modify: `relaykit/dto/channel_settings_test.go`
- Modify: `model/channel_settings_test.go`

- [ ] Add `retry_times` to `dto.ChannelSettings`, define a maximum of three, and reject values outside `0..3` during channel validation.
- [ ] Add deterministic JSON round-trip and validation tests for legacy omission, valid values, negative values, and values above the maximum.
- [ ] Run `cd relaykit && GOWORK=off go test ./dto && GOWORK=off go build ./...` and the model setting tests.

### Task 2: Same-channel relay behavior

**Files:**
- Modify: `controller/relay.go`
- Create: `controller/relay_channel_retry_test.go`

- [ ] Add a retry eligibility function that reuses the existing status-code and skip-retry policy while refusing to retry after `c.Writer.Written()`.
- [ ] Wrap each normal selected-channel attempt in a bounded inner loop. Restore the request body and selected-channel context before each retry, rotate multi-key credentials through the existing setup function, and preserve the existing outer cross-channel retry behavior after same-channel attempts are exhausted.
- [ ] Keep task relay, composite routing, and realtime relay behavior unchanged.
- [ ] Add table tests covering disabled retries, retryable 5xx errors, skip-retry errors, written responses, and specific-channel requests.
- [ ] Run the targeted controller tests and `go test ./controller`.

### Task 3: Channel management form

**Files:**
- Modify: `web/src/features/channels/types.ts`
- Modify: `web/src/features/channels/lib/channel-form.ts`
- Modify: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- Modify: `web/src/features/channels/lib/__tests__/channel-field-update.test.ts`

- [ ] Add a `retry_times` integer field with a default of zero and a maximum of three.
- [ ] Parse and preserve the value in the existing `setting` JSON for create and update payloads.
- [ ] Add the administrator input to Routing Strategy with copy that explains retries exclude the initial request and apply to all channel models.
- [ ] Add form transformation tests for default, configured, and out-of-range values.
- [ ] Run the affected Vitest file, TypeScript typecheck, and lint for the modified frontend files.

### Task 4: Internationalization and verification

**Files:**
- Modify through script: `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`
- Modify: `web/src/i18n/locales/_reports/_sync-report.json`

- [ ] Add the new description through `web/scripts/add-missing-keys.mjs` for all seven locales, run it, remove the temporary script, then run `bun run i18n:sync`.
- [ ] Run `gofmt` on modified Go files, the targeted Go tests, `cd relaykit && GOWORK=off go build ./...`, the targeted frontend test, `bun run typecheck`, and affected-file lint.
- [ ] Review `git diff` to ensure no unrelated user changes were overwritten.

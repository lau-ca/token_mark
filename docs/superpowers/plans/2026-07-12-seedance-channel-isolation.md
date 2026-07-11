# Seedance Channel Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dedicated Seedance task channel whose status polling and content proxy cannot change or expose data from existing channels.

**Architecture:** Register a new channel type and route it to a focused Seedance task adapter derived from the verified upstream contract. Keep platform task IDs public and upstream IDs private, rebuild all user responses from local state, and handle Seedance content redirects inside a credential-safe proxy path selected only by the new channel type.

**Tech Stack:** Go 1.22, Gin, GORM, testify, React 19, TypeScript, Bun, i18next

---

### Task 1: Register the Seedance Channel Type

**Files:**
- Modify: `constant/channel.go`
- Modify: `relay/relay_adaptor.go`
- Modify: `web/default/src/features/channels/constants.ts`
- Modify: `web/default/src/features/channels/lib/channel-utils.ts`
- Modify: `web/classic/src/constants/channel.constants.js`
- Test: `relay/relay_adaptor_test.go`

- [ ] Add `ChannelTypeSeedance` before `ChannelTypeDummy`, append its default base URL, and register the display name `Seedance`.
- [ ] Add an adapter-routing regression test proving Seedance selects its own adapter while Sora and OpenAI continue selecting the Sora adapter.
- [ ] Add the Seedance channel type to both frontend channel selectors and icon/provider mappings without changing existing numeric values.
- [ ] Run the focused Go routing test and frontend type checks.
- [ ] Commit the isolated channel registration.

### Task 2: Implement the Seedance Task Adapter

**Files:**
- Create: `relay/channel/task/seedance/adaptor.go`
- Create: `relay/channel/task/seedance/constants.go`
- Create: `relay/channel/task/seedance/adaptor_test.go`
- Modify: `relay/channel/task/sora/adaptor.go`
- Modify: `relay/channel/task/sora/constants.go`
- Modify: `relay/common/relay_utils.go`

- [ ] Write table tests for supported models, request validation, JSON request forwarding, creation-response parsing, upstream task ID capture, polling status mapping, progress mapping, failure errors, and user response reconstruction.
- [ ] Verify the tests fail before registering the new adapter.
- [ ] Implement the Seedance adapter with `/v1/videos` submission, `/v1/videos/{upstream_task_id}` polling, Bearer authentication, and the existing request-duration bounds.
- [ ] Normalize `completed` and `failed` to terminal progress and preserve 0 through 99 only for non-terminal states.
- [ ] Rebuild OpenAI video responses using the public task ID and platform proxy URL; omit all upstream URL metadata.
- [ ] Remove Seedance models and request-shape branches from the Sora adapter so existing Sora behavior is restored.
- [ ] Run Seedance and Sora adapter tests.
- [ ] Commit the adapter isolation.

### Task 3: Persist Initial State and Protect Terminal Progress

**Files:**
- Modify: `relay/relay_task.go`
- Modify: `controller/relay.go`
- Modify: `model/task.go`
- Modify: `service/task_polling.go`
- Test: `relay/relay_task_test.go`
- Test: `service/task_polling_test.go`

- [ ] Write regression tests proving a Seedance creation result initializes queued/in-progress status and progress, while legacy channel task initialization is unchanged.
- [ ] Write a polling regression test proving terminal success/failure remains `100%` even when an adapter supplies a stale lower progress value.
- [ ] Extend the task submit result with normalized initial task state supplied by the dedicated adapter.
- [ ] Apply initial state only for adapter results that explicitly provide it; retain existing defaults for all legacy channels.
- [ ] Prevent generic progress assignment from overwriting terminal progress.
- [ ] Run the focused relay and polling test suites.
- [ ] Commit status and progress synchronization.

### Task 4: Implement the Seedance Content Proxy

**Files:**
- Modify: `controller/video_proxy.go`
- Create: `controller/seedance_video_proxy_test.go`
- Modify: `relay/video_response_privacy.go`
- Modify: `dto/channel_settings.go`

- [ ] Write HTTP tests for an upstream 302 followed by `206 video/mp4`, Range and If-Range forwarding, redirect limits, invalid schemes, non-media responses, and credential removal across hosts.
- [ ] Write privacy tests proving every upstream URL field is absent from Seedance task responses and the public content URL is used after success.
- [ ] Add a Seedance-only redirect client path that validates each target, strips credentials across hosts, and never relays `Location` to users.
- [ ] Route Seedance content retrieval through the private upstream task ID and preserve 200/206/416 semantics.
- [ ] Keep existing OpenAI/Sora proxy and optional URL-replacement behavior unchanged.
- [ ] Run focused proxy and privacy tests.
- [ ] Commit the safe content proxy.

### Task 5: Frontend Channel Configuration

**Files:**
- Modify: `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- Modify: `web/default/src/features/channels/lib/channel-form.ts`
- Modify: `web/default/src/features/channels/types.ts`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] Add Seedance to the channel form using the existing channel-type composition and accessibility patterns.
- [ ] Make upstream URL hiding intrinsic to Seedance and leave the optional OpenAI/Sora switch scoped to its existing channel types.
- [ ] Add the Seedance label to every maintained locale using the project i18n workflow.
- [ ] Run `bun run i18n:sync`, lint/typecheck, and the production frontend build.
- [ ] Commit frontend configuration support.

### Task 6: Full Verification and Diff Audit

**Files:**
- Verify all modified files.

- [ ] Run `gofmt` on all changed Go files.
- [ ] Run focused Go tests for channel routing, Seedance/Sora adapters, task polling, privacy, proxying, billing, and failure refunds.
- [ ] Run `go test ./relay/... ./controller/... ./service/...` if focused tests pass.
- [ ] Run frontend typecheck/tests/build required by changed files.
- [ ] Run `git diff --check`, inspect every changed file, and verify no generated artifacts, credentials, or unrelated files are staged.
- [ ] Commit the complete implementation if any verification-only fixes remain.

### Task 7: Manual Production Deployment and Channel Migration

**Files:**
- Read only: remote `/root/gateway/work/docker-compose.yml`
- Read only: remote `/root/gateway/master/docker-compose.yml`
- Modify: production channel record for the existing Seedance channel

- [ ] Record current image IDs, container health, restart counts, and the existing Seedance channel record.
- [ ] Back up the current remote compose/config files and channel record without using deployment scripts.
- [ ] Build the production image locally and export it as an archive.
- [ ] Upload and load the image on `192.241.132.211`, then recreate only its Work container and verify health.
- [ ] Upload and load the image on `157.230.213.186`, recreate its Work container, and verify health.
- [ ] Recreate the master container on `157.230.213.186` and verify health.
- [ ] Change only the existing Seedance channel type to `ChannelTypeSeedance`; preserve its URL, key, model list, group, priority, weight, and enabled state.
- [ ] Verify both Work instances and master use the intended image, are healthy, have zero unexpected restarts, and read the migrated channel correctly.
- [ ] Do not access the removed `159.89.150.206` host.
- [ ] Do not call video creation or user-facing task-information endpoints.

# OpenAI Image Response Quality Upstream-First Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the opt-in OpenAI image response normalizer preserve an upstream `quality` field and use the client request value only when the upstream field is missing.

**Architecture:** Keep the existing non-streaming normalization pipeline and change only top-level `quality` resolution in `relay/channel/openai/relay_image.go`. Regression tests cover both generation and editing modes while leaving `created`, `output_format`, `size`, `usage`, streaming, errors, and `data` behavior unchanged.

**Tech Stack:** Go 1.22+, Gin, `gjson`/`sjson`, Testify.

---

### Task 1: Lock the quality precedence contract with tests

**Files:**
- Modify: `relay/channel/openai/image_stream_test.go`

- [ ] **Step 1: Change the conflicting-value case to expect the upstream value**

Update the existing top-level normalization table so an upstream `"quality":"medium"` remains `medium` even when the request contains `high`.

- [ ] **Step 2: Replace request-validation cases with presence and fallback cases**

Cover these exact outcomes for both image generation and image editing:

```text
upstream quality present -> preserve the upstream value
upstream quality missing + request quality present -> copy the request value
upstream quality missing + request quality empty -> use medium
```

- [ ] **Step 3: Run the targeted test and verify the current implementation fails**

Run:

```bash
go test ./relay/channel/openai -run 'TestOpenaiImageHandlerNormalizes(TopLevelFieldsWithoutChangingData|Quality)' -count=1
```

Expected: the conflicting upstream/request case fails because the current implementation always resolves `quality` from the request.

### Task 2: Implement upstream-first quality resolution

**Files:**
- Modify: `relay/channel/openai/relay_image.go`

- [ ] **Step 1: Resolve quality through the existing upstream-first field helper**

Replace the request-only quality resolver with:

```go
quality := imageResponseString(body, "quality", request.Quality, "medium")
```

Remove the now-unused `normalizedImageQuality` helper.

- [ ] **Step 2: Run focused and package tests**

Run:

```bash
go test ./relay/channel/openai -run 'TestOpenaiImageHandlerNormalizes(TopLevelFieldsWithoutChangingData|Quality)' -count=1
go test ./relay/channel/openai -count=1
git diff --check -- relay/channel/openai/relay_image.go relay/channel/openai/image_stream_test.go
```

Expected: all tests pass and `git diff --check` emits no output.

### Task 3: Build and deploy only the production master

**Files:**
- Read only: production master compose and container configuration on `148.113.178.75`
- Create locally: a temporary clean build context and Linux AMD64 image archive

- [ ] **Step 1: Inspect the live master before changing it**

Confirm the active compose path, service name, image reference, container health, local status endpoint, architecture, and the current image ID. Do not restart or edit configuration during inspection.

- [ ] **Step 2: Build a clean Linux AMD64 image**

Create the context from tracked repository content plus only the reviewed quality-change files, build for `linux/amd64`, and export the image as a compressed archive. This prevents unrelated dirty-worktree changes from entering production.

- [ ] **Step 3: Prepare a recoverable rollback point and deploy**

Tag the currently running master image with a timestamped rollback tag, transfer and load the new image, validate the master compose configuration, and recreate only the master service. Do not recreate or restart worker, database, Redis, nginx, or CPA services.

- [ ] **Step 4: Verify production**

Require the master container to be running and healthy, confirm its image ID matches the newly loaded image, and check the local and public `/api/status` endpoints. If verification fails, restore the timestamped rollback image and recreate only the master service.

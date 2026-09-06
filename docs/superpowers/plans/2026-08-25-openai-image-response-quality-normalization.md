# OpenAI Image Response Quality Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make normalized OpenAI image responses return the original client quality for `low`, `medium`, and `high`, while returning `medium` for every other request value.

**Architecture:** Keep the existing non-streaming OpenAI image response normalization boundary. Replace only the generic upstream-first `quality` resolution with a request-only allowlist resolver; leave `created`, `output_format`, `size`, `usage`, and raw `data` handling unchanged.

**Tech Stack:** Go 1.22+, Gin, tidwall/gjson and sjson, testify.

---

### Task 1: Add regression coverage for request-authoritative quality

**Files:**
- Modify: `relay/channel/openai/image_stream_test.go`

- [ ] **Step 1: Change the existing conflicting-value case**

Use an upstream response containing `"quality":"medium"` and a request containing `Quality: "high"`; assert the normalized response contains `"quality":"high"` while upstream `output_format` and `size` remain authoritative.

- [ ] **Step 2: Add a quality resolution table**

Cover these exact request-to-response mappings:

```go
tests := []struct {
    requestQuality string
    wantQuality    string
}{
    {requestQuality: "low", wantQuality: "low"},
    {requestQuality: "medium", wantQuality: "medium"},
    {requestQuality: "high", wantQuality: "high"},
    {requestQuality: "", wantQuality: "medium"},
    {requestQuality: "auto", wantQuality: "medium"},
    {requestQuality: " HIGH ", wantQuality: "medium"},
    {requestQuality: "HIGH", wantQuality: "medium"},
    {requestQuality: "custom", wantQuality: "medium"},
}
```

Run every case for image generation and image editing, with normalization enabled and an upstream `quality` value that conflicts with the expected result.

- [ ] **Step 3: Run the focused test and confirm the old implementation fails**

Run:

```bash
go test ./relay/channel/openai -run 'TestOpenaiImageHandlerNormalizes.*Quality|TestOpenaiImageHandlerNormalizesTopLevelFieldsWithoutChangingData' -count=1
```

Expected before implementation: failure because upstream `quality` still wins.

### Task 2: Implement the quality-specific resolver

**Files:**
- Modify: `relay/channel/openai/relay_image.go`

- [ ] **Step 1: Add the resolver**

Add a package-local function with the exact behavior:

```go
func normalizedImageQuality(requestQuality string) string {
    switch requestQuality {
    case "low", "medium", "high":
        return requestQuality
    default:
        return "medium"
    }
}
```

- [ ] **Step 2: Use the resolver only for quality**

Replace:

```go
quality := imageResponseString(body, "quality", request.Quality, "auto")
```

with:

```go
quality := normalizedImageQuality(request.Quality)
```

Do not change `imageResponseString`, because `output_format` and `size` retain their current upstream-first behavior.

- [ ] **Step 3: Run focused and package tests**

Run:

```bash
go test ./relay/channel/openai -count=1
go test ./relay/common -run TestApplyResponseParamOverrideWithRelayInfo -count=1
```

Expected: all pass.

- [ ] **Step 4: Verify relaykit remains independently buildable**

Run:

```bash
cd relaykit && GOWORK=off go build ./...
```

Expected: exit code 0.

### Task 3: Update the original normalization design

**Files:**
- Modify: `docs/superpowers/specs/2026-08-23-openai-image-response-normalization-design.md`

- [ ] **Step 1: Correct the quality precedence section**

Document that `low`, `medium`, and `high` come from the original client request and all other values normalize to `medium`. Keep upstream-first wording for `created`, `output_format`, and `size` only.

- [ ] **Step 2: Check formatting and scope**

Run:

```bash
git diff --check -- relay/channel/openai/relay_image.go relay/channel/openai/image_stream_test.go docs/superpowers/specs/2026-08-23-openai-image-response-normalization-design.md
```

Expected: no output.

### Task 4: Deploy only the production master

**Files:**
- Deploy source: clean archive of the current repository state containing the quality change
- Remote compose: discover the active master compose path under `/data/services/frimodel-gateway/master` on `148.113.178.75`

- [ ] **Step 1: Inspect production read-only**

Use SSH to identify the running `new-api-master` container, current image ID, compose working directory, compose image name, health state, and current `/api/status` response. Do not touch worker, nginx, CPA, database, or Redis.

- [ ] **Step 2: Build a clean linux/amd64 image locally**

Create a temporary clean context that contains tracked repository files plus only the intentional quality implementation and documentation changes. Build the exact image name used by the remote master and export it as a Docker archive.

- [ ] **Step 3: Prepare rollback and transfer**

Tag the currently running remote master image with a timestamped rollback tag before loading the new image. Upload the archive, verify its SHA-256, and run `docker load`.

- [ ] **Step 4: Recreate only master**

Validate the discovered compose file with `docker compose config --quiet`, then recreate only the `new-api-master` service with `docker compose up -d --force-recreate new-api-master`.

- [ ] **Step 5: Verify production**

Require the container to become healthy, confirm its running image ID equals the newly loaded image, verify the local master `/api/status` and public `https://api.frimodel.com/api/status`, then issue one authenticated 4K `gpt-image-2-adobe` request with `quality: "high"` and confirm the normalized response contains `"quality":"high"`.

- [ ] **Step 6: Preserve rollback details**

Record the rollback image tag and exact compose command needed to restore it. Leave uploaded artifacts in a timestamped remote directory until verification succeeds.

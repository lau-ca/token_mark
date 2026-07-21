# xAI Official Video Image Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Accept official xAI `image.url` and `image.file_id` video inputs while preserving legacy xAI inputs and leaving Seedance request parsing unchanged.

**Architecture:** Separate shared task validation from shared body parsing. Add an xAI-only JSON parser that normalizes the official image union into the existing canonical request, stores file IDs in xAI-local context, and then reuses shared validation. Existing channels continue using the original parser.

**Tech Stack:** Go, Gin, `encoding/json`, reusable request-body storage, focused unit tests.

---

## File Structure

- Modify `relay/common/relay_utils.go` to validate an already parsed `TaskSubmitReq`.
- Create `relay/channel/task/xai/request.go` for xAI-only JSON parsing and image-reference context.
- Modify `relay/channel/task/xai/adaptor.go` to select the xAI parser and serialize URL or file ID.
- Modify `relay/channel/task/xai/adaptor_test.go` for official and legacy xAI inputs.
- Modify `relay/channel/task/seedance/adaptor_test.go` for isolation coverage only.

### Task 1: Add failing request tests

**Files:**
- Modify: `relay/channel/task/xai/adaptor_test.go`
- Modify: `relay/channel/task/seedance/adaptor_test.go`

- [ ] **Step 1: Add accepted xAI cases**

Add table coverage using the existing `buildXAIRequestBody` helper for:

```go
tests := []struct {
	name        string
	requestJSON string
	wantURL     string
	wantFileID  string
}{
	{"public URL object", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":{"url":"https://example.com/frame.png"}}`, "https://example.com/frame.png", ""},
	{"base64 object", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":{"url":"data:image/png;base64,cG5n"}}`, "data:image/png;base64,cG5n", ""},
	{"file ID object", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":{"file_id":"file_123"}}`, "", "file_123"},
	{"legacy string", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":"https://example.com/legacy.png"}`, "https://example.com/legacy.png", ""},
}
```

Decode `payload["image"]` as `map[string]any` and assert `url` or `file_id` exactly.

- [ ] **Step 2: Add rejected xAI cases**

Add a validation helper and require HTTP 400 for:

```go
tests := []string{
	`{"model":"grok-imagine-video","prompt":"animate","image":{}}`,
	`{"model":"grok-imagine-video","prompt":"animate","image":{"url":"https://example.com/a.png","file_id":"file_123"}}`,
	`{"model":"grok-imagine-video","prompt":"animate","image":{"url":"https://example.com/a.png"},"images":["https://example.com/b.png"]}`,
}
```

- [ ] **Step 3: Add Seedance isolation coverage**

Add `TestSeedanceDoesNotAcceptXAIImageObject`, submit a valid Seedance request whose `image` is `{"url":"https://example.com/frame.png"}`, and assert validation still returns HTTP 400.

- [ ] **Step 4: Verify the expected failure**

Run:

```bash
go test ./relay/channel/task/xai ./relay/channel/task/seedance -run 'TestBuildRequestBodyAcceptsOfficialImageInputs|TestValidateRequestRejectsInvalidOfficialImageInputs|TestSeedanceDoesNotAcceptXAIImageObject' -count=1
```

Expected: xAI official object acceptance fails with `invalid_json`; Seedance isolation passes.

### Task 2: Reuse shared validation without sharing xAI parsing

**Files:**
- Modify: `relay/common/relay_utils.go:246-338`

- [ ] **Step 1: Extract the existing post-parse logic**

Change `ValidateMultipartDirect` to parse and delegate:

```go
func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}
	return ValidateParsedTaskRequest(c, info, req)
}
```

Create `ValidateParsedTaskRequest(c, info, req)` by moving the existing normalization, model checks, prompt checks, duration rules, Seedance rules, Sora rules, action selection, and `storeTaskRequest` call without semantic changes.

- [ ] **Step 2: Verify the refactor**

Run:

```bash
go test ./relay/common ./relay/channel/task/seedance ./relay/channel/task/xai -count=1
```

Expected: existing tests pass; xAI official object acceptance still fails.

### Task 3: Implement xAI-only official image parsing

**Files:**
- Create: `relay/channel/task/xai/request.go`
- Modify: `relay/channel/task/xai/adaptor.go`

- [ ] **Step 1: Define the xAI reference and context storage**

```go
type imageRef struct {
	URL    string `json:"url,omitempty"`
	FileID string `json:"file_id,omitempty"`
}

const videoImageRefContextKey = "xai_video_image_ref"
```

Add `setVideoImageRef` and `getVideoImageRef` helpers that store an `imageRef` value in Gin context.

- [ ] **Step 2: Parse the JSON body independently**

Implement `parseVideoTaskRequest(c)`:

```go
var fields map[string]json.RawMessage
if err := common.UnmarshalBodyReusable(c, &fields); err != nil {
	return relaycommon.TaskSubmitReq{}, nil, err
}
imageJSON, hasImage := fields["image"]
delete(fields, "image")
normalizedJSON, err := common.Marshal(fields)
if err != nil {
	return relaycommon.TaskSubmitReq{}, nil, err
}
var req relaycommon.TaskSubmitReq
if err = common.Unmarshal(normalizedJSON, &req); err != nil {
	return relaycommon.TaskSubmitReq{}, nil, err
}
```

If `image` is absent or null, return the canonical request. Otherwise parse it with `parseVideoImageRef`. Reject combinations with `input_reference` or a non-empty `images` array. Copy URL references into `req.Image`; retain file IDs only in the returned `imageRef`.

- [ ] **Step 3: Parse the official union**

`parseVideoImageRef` first tries a non-empty legacy string. Otherwise it decodes `imageRef`, trims both fields, and requires exactly one of `URL` or `FileID`. Invalid shapes return errors such as `image must provide exactly one of url or file_id`.

- [ ] **Step 4: Select xAI parsing for JSON only**

Change `ValidateRequestAndSetAction` so non-JSON requests continue using `ValidateMultipartDirect`. JSON requests call `parseVideoTaskRequest`, store the xAI reference, and call `ValidateParsedTaskRequest`. If the selected input is a file ID, set `info.Action = constant.TaskActionGenerate` after successful validation.

Return local HTTP 400 `invalid_json` errors through `service.TaskErrorWrapperLocal` for malformed xAI input.

- [ ] **Step 5: Serialize URL or file ID upstream**

Move the URL-only `imageRef` definition out of `adaptor.go`. In `BuildRequestBody`, prefer the context reference:

```go
if ref, ok := getVideoImageRef(c); ok {
	payload.Image = ref
} else {
	imageURL, err := a.resolveImageURL(c, req)
	if err != nil {
		return nil, err
	}
	if imageURL != "" {
		payload.Image = &imageRef{URL: imageURL}
	}
}
```

- [ ] **Step 6: Format and run focused tests**

Run:

```bash
gofmt -w relay/common/relay_utils.go relay/channel/task/xai/request.go relay/channel/task/xai/adaptor.go relay/channel/task/xai/adaptor_test.go relay/channel/task/seedance/adaptor_test.go
go test ./relay/channel/task/xai ./relay/channel/task/seedance ./relay/common -count=1
```

Expected: PASS.

### Task 4: Verify task-channel compatibility

**Files:**
- Review only the five files above.

- [ ] **Step 1: Compile every task adaptor**

Run:

```bash
go test ./relay/channel/task/... -run '^$' -count=1
```

Expected: PASS.

- [ ] **Step 2: Check the diff**

Run:

```bash
git diff -- relay/common/relay_utils.go relay/channel/task/xai/request.go relay/channel/task/xai/adaptor.go relay/channel/task/xai/adaptor_test.go relay/channel/task/seedance/adaptor_test.go
git diff --check -- relay/common/relay_utils.go relay/channel/task/xai/request.go relay/channel/task/xai/adaptor.go relay/channel/task/xai/adaptor_test.go relay/channel/task/seedance/adaptor_test.go
```

Expected: no Seedance production change, no unrelated file, and no whitespace error.

- [ ] **Step 3: Report the boundary**

Report that xAI official video image input is complete and not deployed. State that Grok 4.5 output controls remain tracked in `router-for-me/CLIProxyAPI#4464` because the current permission and translator-only repository rule prevent a local source patch.

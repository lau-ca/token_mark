# Seedance Platform Video Proxy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return stable FriModel `/content` URLs for Seedance task queries and stream videos from refreshed private OSS URLs without changing other video channels.

**Architecture:** Seedance polling stores the raw upstream response plus its extracted direct video URL. The Seedance query converter restores the public task identity and replaces public video URLs with the platform content endpoint. A dedicated Seedance content resolver refreshes signed URLs and streams OSS bytes; the shared proxy remains unchanged for other channels.

**Tech Stack:** Go 1.22+, Gin, GORM v2, `net/http`, testify, Docker Compose

---

## File Map

- Modify `relay/channel/task/seedance/adaptor.go` and tests: URL extraction and public response canonicalization.
- Modify `service/task_polling_test.go`: raw JSON plus private OSS URL persistence.
- Modify `model/task.go` and tests: focused private ResultURL update.
- Create `controller/video_proxy_seedance.go` and tests: Seedance URL refresh and OSS streaming.
- Modify `controller/video_proxy.go` and tests: delegate Seedance only; preserve every other channel.

### Task 1: Extract private URLs and rewrite public Seedance responses

**Files:**
- Modify: `relay/channel/task/seedance/adaptor.go`
- Modify: `relay/channel/task/seedance/adaptor_test.go`

- [ ] **Step 1: Add failing URL-priority tests**

Add table tests for `metadata.final_video_url`, root `video_url`, root `url`, `metadata.origin_video_url`, and `metadata.url`. Each calls `ParseTaskResult` and asserts `TaskInfo.Url`.

```go
func TestSeedanceParseTaskResultExtractsVideoURL(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"final metadata", `{"status":"completed","metadata":{"final_video_url":"https://oss.example/final.mp4"}}`, "https://oss.example/final.mp4"},
		{"root video", `{"status":"completed","video_url":"https://oss.example/video.mp4"}`, "https://oss.example/video.mp4"},
		{"root URL", `{"status":"completed","url":"https://oss.example/root.mp4"}`, "https://oss.example/root.mp4"},
		{"origin metadata", `{"status":"completed","metadata":{"origin_video_url":"https://oss.example/origin.mp4"}}`, "https://oss.example/origin.mp4"},
		{"metadata URL", `{"status":"completed","metadata":{"url":"https://oss.example/meta.mp4"}}`, "https://oss.example/meta.mp4"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(test.body))
			require.NoError(t, err)
			assert.Equal(t, test.want, result.Url)
		})
	}
}
```

- [ ] **Step 2: Add a failing public response test**

Use a completed task containing OSS URLs and `metadata.cost_credits`. Assert `id` and `task_id` are the public ID; root and nested URL fields equal `https://api.frimodel.com/v1/videos/task_public/content`; non-URL metadata remains; OSS hostname and signature are absent.

- [ ] **Step 3: Verify failure**

```bash
go test ./relay/channel/task/seedance -run 'TestSeedance(ParseTaskResultExtractsVideoURL|ConvertTaskUsesPlatformContentURL)$' -count=1
```

Expected: FAIL because URL extraction is absent and the converter returns raw bytes.

- [ ] **Step 4: Extend the response model and extraction**

Add `URL`, `VideoURL`, and `Metadata` to `responseTask`. Add exported `ExtractVideoURL(body []byte) (string, error)` using the tested priority. Set `TaskInfo.Url` only for successful responses.

```go
type responseTask struct {
	ID string `json:"id"`
	TaskID string `json:"task_id,omitempty"`
	Object string `json:"object"`
	Model string `json:"model"`
	Status string `json:"status"`
	Progress int `json:"progress"`
	CreatedAt int64 `json:"created_at"`
	CompletedAt int64 `json:"completed_at,omitempty"`
	Seconds string `json:"seconds,omitempty"`
	URL string `json:"url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Error *dto.OpenAIVideoError `json:"error,omitempty"`
}
```

- [ ] **Step 5: Canonicalize the public response**

Parse `task.Data` into `map[string]any`, set public identity/model/status/progress/timestamps, and remove URLs for non-success tasks. For success, set root `url` and `video_url` to `taskcommon.BuildProxyURL(task.TaskID)` and recursively replace HTTP/HTTPS strings inside metadata while preserving non-URL values.

```go
func replaceSeedanceMetadataURLs(value any, proxyURL string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = replaceSeedanceMetadataURLs(child, proxyURL)
		}
	case []any:
		for index, child := range typed {
			typed[index] = replaceSeedanceMetadataURLs(child, proxyURL)
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			return proxyURL
		}
	}
	return value
}
```

- [ ] **Step 6: Run and commit**

```bash
go test ./relay/channel/task/seedance -count=1
git add relay/channel/task/seedance/adaptor.go relay/channel/task/seedance/adaptor_test.go
git commit -m "fix: proxy Seedance task result URLs"
```

Expected: PASS.

### Task 2: Verify polling persists raw JSON and private OSS URL

**Files:**
- Modify: `service/task_polling_test.go`

- [ ] **Step 1: Strengthen the existing fixture**

Make `rawSeedancePollingAdaptor.ParseTaskResult` return a successful `TaskInfo` with an OSS URL. In `TestUpdateVideoSingleTaskPreservesRawSeedanceResponse`, assert exact raw `task.Data` and exact `task.PrivateData.ResultURL`.

- [ ] **Step 2: Run and commit**

```bash
go test ./service -run TestUpdateVideoSingleTaskPreservesRawSeedanceResponse -count=1
git add service/task_polling_test.go
git commit -m "test: persist Seedance OSS result URLs"
```

Expected: PASS with no production change because polling already stores non-empty `TaskInfo.Url` privately.

### Task 3: Add a focused private ResultURL update

**Files:**
- Modify: `model/task.go`
- Modify: `model/task_cas_test.go`

- [ ] **Step 1: Add a failing persistence test**

Create a completed task with upstream ID, billing context, quota, status, and data. Call `UpdateResultURL`, reload it, and assert only `PrivateData.ResultURL` changed.

- [ ] **Step 2: Implement the update**

```go
func (t *Task) UpdateResultURL(resultURL string) error {
	t.PrivateData.ResultURL = resultURL
	return DB.Model(t).Select("private_data").Updates(t).Error
}
```

- [ ] **Step 3: Run and commit**

```bash
go test ./model -run TestTaskUpdateResultURLPreservesTaskState -count=1
git add model/task.go model/task_cas_test.go
git commit -m "feat: update private task result URL"
```

Expected: PASS on the cross-database GORM path.

### Task 4: Build the Seedance-only OSS resolver and streamer

**Files:**
- Create: `controller/video_proxy_seedance.go`
- Create: `controller/video_proxy_seedance_test.go`
- Modify: `controller/video_proxy.go`
- Modify: `controller/video_proxy_test.go`

- [ ] **Step 1: Add failing controller tests**

Use deterministic `httptest.Server` fixtures for these contracts:

- Valid stored OSS URL receives Range but no Authorization.
- Expired `Expires` query refreshes upstream before OSS fetch.
- Historical platform `/content` ResultURL refreshes upstream.
- OSS `403` refreshes once and retries once.
- Cross-host redirects strip Authorization, cookies, and API-key headers.
- Successful non-video content returns `502`.
- Existing OpenAI, Sora, XAI, Gemini, and Vertex tests remain unchanged.

- [ ] **Step 2: Implement URL usability**

```go
func seedanceResultURLIsUsable(resultURL string, taskID string) bool {
	parsed, err := url.Parse(strings.TrimSpace(resultURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	if strings.Contains(parsed.Path, "/v1/videos/"+taskID+"/content") {
		return false
	}
	if value := parsed.Query().Get("Expires"); value != "" {
		expiresAt, err := strconv.ParseInt(value, 10, 64)
		if err != nil || expiresAt <= time.Now().Add(time.Minute).Unix() {
			return false
		}
	}
	return true
}
```

- [ ] **Step 3: Implement upstream refresh**

Use `relay.GetTaskAdaptor`, `FetchTask`, and `ParseTaskResult` with the private upstream ID and channel key. Require success plus a usable direct URL, call `task.UpdateResultURL`, and return it. Never send the channel key to OSS.

- [ ] **Step 4: Implement streaming and one retry**

`proxySeedanceVideo` forwards `Range` and `If-Range`, validates the source and every redirect, strips sensitive headers on host changes, accepts `200`, `206`, and `416`, copies video response headers, rejects non-video success, and uses `Cache-Control: private, no-store`. On first `401`, `403`, or `404`, refresh once and retry once.

- [ ] **Step 5: Isolate Seedance in the shared controller**

After channel lookup:

```go
if channel.Type == constant.ChannelTypeSeedance {
	proxySeedanceVideo(c, task, channel)
	return
}
```

Remove Seedance from the remaining generic URL-construction, redirect, and content-type branches. Do not change any other channel behavior.

- [ ] **Step 6: Run and commit**

```bash
go test ./controller -run 'TestSeedanceVideoProxy|TestVideoProxyTrustedChannelContentBypassesSSRF|TestVideoProxyUntrustedResultURLRemainsSSRFProtected|TestVideoProxyPrivacyForwardsRangesAndFiltersResponseHeaders' -count=1
git add controller/video_proxy.go controller/video_proxy_test.go controller/video_proxy_seedance.go controller/video_proxy_seedance_test.go
git commit -m "fix: stream Seedance videos from OSS"
```

Expected: PASS.

### Task 5: Full verification and deployment

**Files:**
- Verify all backend changes.
- Deploy to `/data/frimodel/{worker,master}` on `148.113.178.75`.

- [ ] **Step 1: Format and test everything**

```bash
gofmt -w relay/channel/task/seedance/adaptor.go relay/channel/task/seedance/adaptor_test.go service/task_polling_test.go model/task.go model/task_cas_test.go controller/video_proxy.go controller/video_proxy_test.go controller/video_proxy_seedance.go controller/video_proxy_seedance_test.go
go test ./... -count=1
git diff --check
git status --short
```

Expected: all tests pass and unrelated worktree changes remain untouched.

- [ ] **Step 2: Build and transfer linux/amd64 image**

Build from committed HEAD plus only intentionally uncommitted production changes already present in the running image. Save `frimodel/new-api:latest` plus a timestamped release tag, verify SHA-256, and upload to `/data/transfers/new-api/seedance-platform-proxy-<timestamp>`.

- [ ] **Step 3: Load with rollback tag and deploy worker first**

Tag the current image `pre-seedance-platform-proxy-<timestamp>`, load the new image, recreate `/data/frimodel/worker`, wait for healthy, and verify `https://api.frimodel.com/api/status` returns `200`.

- [ ] **Step 4: Deploy master last**

Recreate `/data/frimodel/master`, wait for healthy, and verify `https://platform.frimodel.com/api/status` returns `200`.

- [ ] **Step 5: Run live contract test**

Create a new `videos-mini` 480p task with the supplied test token. Poll to completion and verify public IDs plus FriModel `/content` URLs with no OSS signature. Request `/content` with `Range: bytes=0-1023`; require `206`, `video/mp4`, valid Content-Range, and MP4 bytes.

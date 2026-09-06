# xAI Image Aspect Ratio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Forward dynamic image aspect ratios through the xAI image adaptor.

**Architecture:** Reuse `dto.ImageRequest.Extra` for the provider-specific input and add the corresponding field only to the xAI upstream DTO. Decode through the project's JSON wrapper and omit invalid or empty values.

**Tech Stack:** Go, Gin, testify, project JSON wrapper

---

### Task 1: Protect aspect-ratio conversion behavior

**Files:**
- Create: `relay/channel/xai/adaptor_test.go`

- [x] Add a table test that converts requests containing `16:9`, `9:16`, `4:3`, and `3:4`, then asserts the xAI DTO preserves each value.
- [x] Add cases proving missing, empty, and non-string values are omitted.
- [x] Run `go test ./relay/channel/xai` and confirm the supported-ratio cases fail before implementation.

### Task 2: Forward the xAI aspect ratio

**Files:**
- Modify: `relay/channel/xai/dto.go`
- Modify: `relay/channel/xai/adaptor.go`

- [x] Add `AspectRatio string` with `json:"aspect_ratio,omitempty"` to the xAI image request DTO.
- [x] Decode `request.Extra["aspect_ratio"]` with `common.Unmarshal` and assign only non-empty strings.
- [x] Remove the outdated comment claiming xAI does not support image sizing controls.
- [x] Run `gofmt` on changed Go files.
- [x] Run `go test ./relay/channel/xai` and confirm all cases pass.

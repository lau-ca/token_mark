# Qianfan K-Series Integration Design

## Goal

Complete Baidu Qianfan K-series support through the existing Baidu V2 channel without changing any other channel's request, polling, or billing behavior.

## Scope

- Generation models: `K3.0`, `K3O`, and `k3.0-turbo`.
- Fixed capability models required by the official APIs: `K-Identify-Face` and `K-Advanced-Lip-Sync`.
- Native Qianfan operations: `text2video`, `img2video`, `omni-video`, `motion-control`, `lip-sync`, and `advanced-lipsync`.
- Existing task query flow through the stored public task ID.
- Subject and voice create, list, query, preset-list, and delete operations through the existing channel-admin proxy.
- Request-aware expression billing for generation and advanced operations.

## Architecture

Keep `ChannelTypeBaiduV2` and the existing Qianfan task adaptor. Native requests sent to `/qianfan/v1/videos` retain the official Qianfan body shape and are forwarded to `/beta/video/generations/qianfan-video`. The adaptor only extracts a bounded billing projection into `TaskSubmitReq`; it does not rewrite advanced provider fields.

The standard `/v1/videos` compatibility route remains available. It emits the direct K3.0 body shape for standard K3.0 and the nested `contents/settings` body shape only for `k3.0-turbo`.

The synchronous `K-Identify-Face` operation returns the upstream response directly and records a terminal internal task using `session_id` as the upstream identifier. Asynchronous operations retain the existing public task ID and polling flow.

## Request Validation and Billing Projection

- Accept only documented operation names.
- Enforce documented model-operation combinations.
- Bound every supplied duration with `relaycommon.MaxTaskDurationSeconds` before billing.
- Normalize `sound` from the documented `on`/`off` strings, while retaining boolean compatibility.
- Detect reference videos from `video_list` and compatible legacy fields.
- Detect voices from `voice_list` and compatible legacy fields.
- Derive advanced lip-sync duration from its selected audio intervals when available; otherwise use the minimum billable interval so pre-consume cannot be zero.
- Motion control uses a minimum billable duration for pre-consume when the request has no explicit duration, then uses upstream final deduction for settlement when supplied.

## Response and Settlement

Qianfan response parsing reads both `final_unit_deduction` and `usage.credits`. Values are converted with the shared checked quota helpers and never by an unchecked integer cast. The Qianfan adaptor may return the final quota from the upstream deduction; the shared polling settlement continues to skip traditional fixed per-call tasks, but permits an adaptor-provided final amount for Qianfan expression-billed tasks.

Failed asynchronous tasks retain the existing refund path. Successful tasks settle exactly once through the existing terminal-state CAS guard.

## Subject and Voice Resources

Keep these operations admin-only and scoped by Baidu V2 channel ID. Correct the official identifiers and pagination contracts:

- Subject list accepts `Custom-Elements` and `Presets-Elements`, with `pageNum` 1..1000 and `pageSize` 1..500.
- Custom voice query/list use `Custom-Voice`.
- Custom and preset voice lists use `pageNum` 1..1000 and `pageSize` 1..1000, defaulting to 1 and 30.
- Creation and deletion retain the official request bodies.

## Isolation

No channel type, global route behavior, database schema, or frontend component changes are required. Shared task settlement changes are guarded by adaptor output and preserve the existing skip behavior for every other per-call and expression-billed task.

## Verification

Add deterministic tests for model/type validation, native passthrough, K3.0 versus Turbo conversion, billing projection, synchronous face identification, final deduction parsing, resource model constants, and pagination bounds. Run focused package tests, formatting, `go test` for affected packages, and a final diff review against the pre-existing dirty worktree.

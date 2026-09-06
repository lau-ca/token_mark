# Onfishes Seedance Compatibility Design

## Goal

Allow an administrator to register the onfishes Seedance service as the existing Doubao Video channel and expose its documented Seedance video and asset-library APIs through New API, without changing the behavior of the official VolcEngine channel.

## Recommended Approach

Extend the existing Doubao Video task adaptor because it already owns the Seedance 1.x/2.x model list, token-based task billing, asynchronous task persistence, polling, and OpenAI-compatible video responses. Do not reuse the generic Seedance, Sora, OpenAI, or VolcEngine adaptors: their upstream paths and response contracts differ, and changing them would affect existing channels.

Use channel type, not hostname matching, to select the upstream route:

- Doubao Video: `/v3/contents/generations/tasks`
- VolcEngine: `/api/v3/contents/generations/tasks`

This keeps custom base URLs supported and prevents an onfishes-specific hostname from being embedded in business logic.

## Verified Documentation Scope

The left navigation contains sixteen interfaces, all of which are in scope:

- four video-task interfaces;
- two liveness verification interfaces;
- five asset interfaces;
- five asset-group interfaces.

All sixteen compatibility routes keep New API's existing authentication behavior and require `Authorization: Bearer <API_KEY>`. The documented `api-key` alias is intentionally not added.

## Public API Surface

Add aliases matching the four documented video-task paths:

- `POST /v3/contents/generations/tasks`
- `GET /v3/contents/generations/tasks/:task_id`
- `GET /v3/contents/generations/tasks`
- `DELETE /v3/contents/generations/tasks/:task_id`

Creation and single-task retrieval reuse the existing task relay, ownership checks, billing, and polling. The `/v3` compatibility routes use native Seedance response builders rather than the existing OpenAI-compatible response builder:

- creation returns `{ "id": "<upstream_task_id>" }`;
- single retrieval returns the documented task object directly, without a `{code,data}` wrapper;
- list returns `{ "total": number, "items": [...] }` directly;
- deletion returns the latest documented task object directly.

Task listing reads the authenticated user's locally stored Doubao Video tasks and supports `page_num`, `page_size`, `filter.status`, `filter.task_ids`, `filter.model`, and `filter.service_tier`. It may return upstream task IDs, but it must never return tasks belonging to another user.

Cancellation resolves the upstream task ID against the authenticated user's stored task, verifies ownership and channel type, forwards `DELETE`, and updates the local task only after the upstream accepts cancellation or deletion. Completed upstream tasks may be deleted upstream while their local audit and billing record remains retained. The operation does not refund already settled usage unless the existing task billing lifecycle already treats the task as unsettled.

Keep the existing `/v1/videos`, `/v1/video/generations`, and related routes unchanged.

## Request Conversion

The Doubao adaptor accepts both the existing New API task request and the documented native Seedance body.

- Preserve a native `content` array containing `text`, `image_url`, `video_url`, `audio_url`, and `draft_task` items, including documented roles such as `first_frame`, `last_frame`, `reference_image`, `reference_video`, and `reference_audio`.
- Continue converting `prompt`, `images`, `referenceVideos`, and `referenceAudios` into `content` when callers use the existing unified request.
- Map top-level `duration`, `resolution`, `ratio`, and other documented optional Seedance fields directly.
- Apply `metadata` last only for fields that were not explicitly supplied at the top level, so compatibility metadata remains useful without unexpectedly overriding normal request fields.
- Preserve explicit scalar zero and false values with pointer-backed optional fields.
- Validate `execution_expires_after` as 3600 through 259200 and `priority` as 0 through 9 when present.
- Validate `resolution` and `ratio` against the documented values.
- Accept the documented automatic-duration sentinel `duration=-1`; otherwise require an integer from 4 through 15. The sentinel must not become a negative billing multiplier.
- Require a non-empty model and at least one meaningful `text`, `image_url`, or `video_url` content item, matching the documented minimum.

The model name continues through the existing model-mapping system, allowing the administrator to expose `Doubao-Seedance-2.0` while mapping it to the exact upstream model identifier if needed.

## Response and Status Handling

Expand the stored upstream response type so the adaptor retains and returns the documented fields: `id`, `model`, `status`, `content.video_url`, `content.last_frame_url`, `seed`, `resolution`, `ratio`, `duration`, `framespersecond`, `priority`, `draft`, `usage`, `created_at`, `updated_at`, `generate_audio`, `service_tier`, `execution_expires_after`, and error data.

Normalize statuses as follows:

| Upstream status | Local status |
| --- | --- |
| `pending`, `queued` | queued |
| `processing`, `running` | in progress |
| `succeeded` | success |
| `failed`, `cancelled`, `expired` | failure |

Native `/v3` responses use the upstream task ID and documented upstream status vocabulary. Video and last-frame URLs are returned as provided by onfishes, matching the documented response. The existing `/v1/videos` and `/v1/video/generations` routes retain their current platform task-ID behavior.

Tasks created through `/v3/contents/generations/tasks` store the upstream ID as their local lookup ID, scoped by user ownership. This allows subsequent documented query, list, and delete calls to use the same ID without database-specific JSON searches. Tasks created through existing `/v1` routes continue using generated platform IDs.

## Asset Library APIs

Reuse `SeedanceAssetProxy` for all twelve documented liveness, asset, and asset-group actions. Add `/v1/volc/ark` as a channel-backed route while retaining the legacy `/volc/ark` environment-backed route for compatibility.

All twelve actions use `POST /v1/volc/ark?Action=<action>&Version=2024-01-01`:

- `CreateVisualValidateSession`, `GetVisualValidateResult`;
- `CreateAsset`, `GetAsset`, `ListAssets`, `UpdateAsset`, `DeleteAsset`;
- `CreateAssetGroup`, `GetAssetGroup`, `ListAssetGroups`, `UpdateAssetGroup`, `DeleteAssetGroup`.

The documented `/v1/volc/ark` route selects a registered Doubao Video channel through the internal billing model `seedance-asset-library`. It uses that channel's base URL, key, proxy setting, multi-key rotation, group, and health/accounting context. The administrator therefore registers one Doubao Video channel with:

- base URL `https://ai-api.onfishes.com`;
- the real onfishes API key;
- the desired Seedance video model names;
- the internal `seedance-asset-library` capability and per-call price.

The proxy appends `/v1/volc/ark` to the registered channel base URL. Client credentials are never forwarded upstream. The action allowlist, request body, complete query string including `Version`, response status, safe headers, pre-consume, settlement, and failure refund behavior remain unchanged. Onfishes remains responsible for its documented ID translation (`zw-` and `zwg-`), ProjectName injection, liveness session handling, and response shapes; New API transparently preserves those bodies and responses.

The old `/volc/ark` route keeps supporting `SEEDANCE_ASSET_PROXY_BASE_URL` and `SEEDANCE_ASSET_PROXY_API_KEY` so an existing deployment is not broken, but those variables are not required for the new documented route.

## Error Handling and Isolation

- Reject malformed requests with `400` before any upstream request.
- Return controlled errors for missing tasks, wrong ownership, unsupported channel types, and invalid cancellation state.
- Preserve the upstream status code and safe error body for accepted proxy operations.
- Do not alter the official VolcEngine route, other task adaptors, existing video privacy behavior, or unrelated frontend/model-capability work already present in the dirty worktree.

## Verification

Add deterministic tests covering:

- channel-specific create and fetch paths;
- native and unified request conversion, including media inputs and explicit false/zero values;
- `draft_task`, existing Bearer authentication, documented enum/range validation, and `duration=-1`;
- cancelled and expired statuses;
- exact native creation, single-task, list, and deletion response shapes;
- upstream task-ID, video URL, and last-frame response preservation on `/v3` routes;
- route registration for the four video paths and the asset alias;
- task-list ownership filtering;
- cancellation forwarding with the upstream ID and local state update.

Run focused Go tests for the touched packages. After implementation, revisit every left-menu page in the onfishes documentation and compare its method, path, request fields, response fields, authentication, and status semantics with the implemented behavior. Do not send a paid generation request during documentation verification.

# Seedance Platform Video Proxy Design

## Goal

Expose stable FriModel video URLs for Seedance tasks while keeping upstream OSS addresses private. Seedance task queries return the platform `/content` URL, and the authenticated `/content` endpoint fetches the current OSS object and streams its video bytes.

This behavior is limited to `ChannelTypeSeedance`. Other video channels retain their current query and content-resolution behavior.

## Public API Contract

### Create task

`POST /v1/videos` continues to return the platform public task ID:

```json
{
  "id": "task_public",
  "task_id": "task_public",
  "object": "video",
  "model": "videos-mini",
  "status": "queued",
  "progress": 20
}
```

The upstream task ID remains private.

### Query task

`GET /v1/videos/{public_task_id}` uses the stored Seedance task result for status, timestamps, progress, errors, and non-URL metadata. It canonicalizes `id` and `task_id` to the public task ID.

When the task is completed, every HTTP or HTTPS URL value in the root `url`, root `video_url`, and nested `metadata` values is replaced with:

```text
https://api.frimodel.com/v1/videos/{public_task_id}/content
```

Non-URL metadata such as expiry, cost, dimensions, or provider status is preserved. Queued, in-progress, and failed responses do not expose upstream URLs.

### Read video content

`GET /v1/videos/{public_task_id}/content` remains authenticated and verifies task ownership and completion before reading video data.

For Seedance it resolves the video source in this order:

1. Use the privately stored direct OSS URL when present.
2. If the URL is absent, is already the platform proxy URL, is expired, or OSS rejects it with an authorization/expiry response, query the Seedance upstream task endpoint using the private upstream task ID and channel key.
3. Extract the refreshed URL using this priority: `metadata.final_video_url`, `video_url`, `url`, `metadata.origin_video_url`, `metadata.url`.
4. Save the refreshed URL in `TaskPrivateData.ResultURL` for subsequent requests.
5. Fetch the OSS URL without forwarding the channel Authorization header and stream the response to the client.

The handler supports `Range`, `If-Range`, `200`, `206`, `416`, `Content-Type`, `Content-Length`, `Content-Range`, `Accept-Ranges`, `ETag`, and `Last-Modified`. It rejects non-video successful responses.

## Data Flow

During polling, the complete Seedance upstream JSON remains stored in `task.Data`. `ParseTaskResult` additionally extracts the direct result URL into `TaskInfo.Url`, causing the polling service to store it in `task.PrivateData.ResultURL`.

The public query converter reads `task.Data`, restores the public identity and task state, and replaces video URLs only in the outgoing response. It never overwrites the private raw response with platform URLs.

The content handler reads the private URL. Refreshing an expired URL updates only private task data; it does not change billing, task status, or settlement state.

## Security Boundaries

- Public endpoints never expose the Seedance channel key.
- Query responses never expose OSS signatures or the upstream task ID.
- The content request to OSS never contains `Authorization`, cookies, proxy credentials, or API-key headers from the upstream task query.
- Direct OSS URLs still pass URL and SSRF validation before fetching.
- Redirects are limited, require HTTP or HTTPS, are revalidated, and strip sensitive headers when hosts change.
- The existing token-or-user authentication and task ownership checks remain mandatory.

## Historical Tasks

Historical tasks may have sanitized `task.Data` and no direct OSS URL. `/content` attempts to recover them by querying the upstream task endpoint with the stored private upstream task ID. They work if the upstream still retains the task and can issue a new signed URL. If the upstream has deleted the result, the endpoint returns a controlled upstream-content error.

## Nginx Role

Nginx continues routing `/v1/` to the New API worker with buffering and request buffering disabled. It does not resolve database tasks, refresh OSS signatures, or proxy OSS independently. The Go service owns authentication, URL refresh, SSRF checks, and streaming.

## Code Scope

- Extend the Seedance response type and URL extraction logic.
- Restore Seedance-only public response canonicalization without enabling it for other channel types.
- Add a focused Seedance OSS resolver used by the shared video proxy.
- Change only the Seedance branch of `VideoProxy`; preserve OpenAI, Sora, Gemini, Vertex, XAI, and other channel behavior.
- Keep the raw Seedance polling response in private task storage.
- Remove the superseded raw-OSS public response contract and tests.

## Error Handling

- Malformed Seedance task JSON returns the existing conversion error.
- Missing or invalid video URLs trigger one upstream refresh attempt.
- OSS authorization or expiry failures trigger one refresh-and-retry cycle; other upstream failures are returned as a controlled `502`.
- An upstream refresh that reports a non-completed task or no result URL returns a controlled `502` without changing task billing or status.
- Invalid Range behavior continues to return `416` with `Content-Range` when supplied upstream.

## Verification

- Seedance query tests assert public IDs, platform `/content` URLs, preserved non-URL metadata, and absence of OSS signatures.
- Seedance polling tests assert raw upstream JSON and private OSS URL persistence.
- Seedance content tests cover direct OSS streaming, Range headers, expired URL refresh, historical-task recovery, non-video rejection, and credential stripping.
- Existing OpenAI/Sora, Gemini/Vertex, XAI, unsupported-channel, SSRF, and data-URL proxy tests must remain unchanged and pass.
- Run all backend Go tests before deployment.
- Deploy worker first, verify health and API status, then deploy master and verify platform status.

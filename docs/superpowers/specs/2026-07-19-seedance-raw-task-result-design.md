# Seedance Raw Task Result Design

## Goal

Return the Seedance upstream task result without hiding or rewriting its OSS video URLs, while retaining the existing authenticated `/v1/videos/{task_id}/content` endpoint for clients that want video bytes instead of an upstream URL.

## Public API Contract

### Create task

`POST /v1/videos` continues to return the platform's public task ID so clients have a stable ID for later queries and content access. Because the query response is intentionally raw, its body may subsequently include the upstream task ID alongside the OSS fields.

### Query task

`GET /v1/videos/{task_id}` returns the upstream Seedance task JSON without removing or replacing fields. This includes `url`, `video_url`, and URL values under `metadata` such as `final_video_url` or `origin_video_url`.

The endpoint does not replace these fields with `/v1/videos/{task_id}/content` and does not normalize upstream progress, timestamps, metadata, or error fields beyond the existing HTTP transport behavior.

### Read video content

`GET /v1/videos/{task_id}/content` remains available and returns video bytes. It continues to enforce task ownership, require a completed task, support `Range` requests, forward video response headers, and reject non-video upstream responses.

The content endpoint is an optional compatibility path. Clients may instead use the OSS URL returned by the query endpoint.

## Code Changes

- Remove Seedance from the response-privacy rules that always replace upstream URLs with the platform content URL.
- Make the Seedance OpenAI-video query conversion return the stored upstream task response rather than constructing a sanitized response.
- Preserve the raw upstream response in task data during polling so later queries can return it.
- Keep Seedance-specific `/content` streaming behavior and its credential-safe redirect handling.
- Delete tests whose contract requires Seedance query responses to hide upstream IDs or URLs.
- Add focused regression coverage proving that completed Seedance queries retain OSS URL fields and that `/content` still streams video bytes.

## Data and Security Boundaries

The upstream API key and channel configuration remain private. Returning the upstream response intentionally exposes only values supplied in the upstream task response, including its temporary OSS signature. The `/content` handler must continue removing authorization and cookie headers when a redirect crosses hosts so channel credentials are never sent to OSS.

## Error Handling

- Upstream query errors continue through the existing task polling failure path.
- Malformed stored task JSON produces the existing conversion error rather than a fabricated completed response.
- `/content` keeps its current errors for missing tasks, incomplete tasks, blocked URLs, invalid content types, upstream failures, and invalid ranges.

## Verification

- Seedance adaptor tests cover raw completed responses containing `url`, `video_url`, and nested metadata URLs.
- Response-privacy tests confirm Seedance is no longer forced through URL replacement.
- Existing Seedance content proxy tests continue to cover redirects, credential stripping, Range behavior, and video content-type validation.
- Run targeted Go tests for the Seedance adaptor, relay response privacy, task polling, and video proxy packages.

# Seedance Channel Isolation Design

## Goal

Integrate `videos-fast`, `videos-mini`, and `videos-standard` as a dedicated Seedance channel without changing the behavior of existing OpenAI, Sora, or other task channels.

The integration must expose platform task IDs and platform download URLs only. Upstream task IDs and every upstream URL field remain private.

## Verified Upstream Contract

Existing paid tasks were queried directly without creating new tasks.

- `GET /v1/videos/{upstream_task_id}` returns `queued`, `in_progress`, `completed`, or `failed` with an integer progress value.
- Terminal responses return progress `100`.
- Completed responses contain upstream URLs in `url`, `video_url`, and multiple metadata fields.
- `GET /v1/videos/{upstream_task_id}/content` returns `302` to a separate content host.
- Following the redirect returns `video/mp4` and supports byte ranges with `206 Partial Content`.

## Architecture

Add a dedicated Seedance channel type and task adapter. The existing Sora adapter returns to handling only its original models and protocol.

The Seedance adapter owns:

- supported model registration;
- request validation and upstream request serialization;
- creation-response parsing and private upstream task ID capture;
- polling URL construction and response parsing;
- status and progress normalization;
- OpenAI-compatible user response construction with platform IDs and URLs.

The database continues to store:

- the platform-generated public task ID in `task_id`;
- the real upstream task ID in private task data;
- sanitized upstream response data that cannot expose upstream URLs to users.

## Status and Progress

Polling uses the real upstream task ID and maps statuses as follows:

| Upstream | Platform |
| --- | --- |
| `queued`, `pending` | queued |
| `processing`, `in_progress` | in progress |
| `completed` | success |
| `failed`, `cancelled` | failure |

Non-terminal progress accepts values from 0 through 99. Success and failure always store `100%`, regardless of a contradictory upstream progress field. This prevents a later generic assignment from overwriting terminal progress.

The creation response initializes the local task from the upstream status and progress instead of always starting as unknown and zero.

## User Response Privacy

Seedance user responses are rebuilt from local task state rather than forwarding upstream JSON.

- `id` and `task_id` use the platform public task ID.
- `model`, `status`, `progress`, timestamps, duration, and errors use normalized local values.
- Before completion, `url`, `video_url`, and URL metadata are empty.
- After completion, all downloadable URL fields point to `/v1/videos/{public_task_id}/content` on the platform.
- Upstream URL-like strings are not returned in metadata, errors, task data, result URL, or failure reason.

Admin-only private task data retains the upstream task ID required for polling and content retrieval.

## Video Content Proxy

The content endpoint resolves the public task ID to the private upstream task ID and calls the Seedance upstream content endpoint.

Redirect handling is specific to Seedance:

- follow a small fixed number of redirects;
- allow only HTTP and HTTPS targets;
- remove `Authorization` and other upstream credentials when the host changes;
- preserve `Range` and `If-Range`;
- accept only `200`, `206`, and correctly formed `416` responses;
- stream approved media response headers and body to the caller;
- expose no redirect location or upstream URL to the user;
- use private, no-store caching headers at the platform boundary.

## Isolation

- Existing OpenAI and Sora channel type constants and adapter routing remain unchanged.
- Seedance model-specific branches are removed from the Sora adapter.
- Existing video URL replacement behavior for old channels remains unchanged.
- Only channels explicitly configured with the new Seedance type enter this adapter and redirect policy.
- Existing task billing, refund, and settlement infrastructure remains shared; Seedance continues using its configured billing expression.

## Production Migration

The existing Seedance channel is changed from Sora to the new Seedance type only after all Work and master instances run the new build. Its base URL, key, models, group, priority, weight, and billing configuration remain unchanged.

Deployment is manual and sequential:

1. build and package the image locally;
2. back up current remote compose/configuration and record the current image;
3. update `192.241.132.211` Work;
4. update `157.230.213.186` Work;
5. update `157.230.213.186` master;
6. update the production channel type;
7. verify container health, restart counts, image identity, and channel configuration.

The removed `159.89.150.206` host is not accessed.

## Verification

Code verification covers:

- Seedance status and progress parsing, including terminal progress enforcement;
- public/private task ID separation;
- removal and replacement of all upstream URL fields;
- redirect handling, credential stripping, range forwarding, and media response validation;
- adapter routing isolation from Sora and OpenAI;
- unchanged legacy adapter behavior;
- existing billing and failure-refund tests.

Per request, deployment verification does not create a video task and does not call the user-facing video creation or task-information endpoints.

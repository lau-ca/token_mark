# Internal Channel Video Proxy Fetch Design

## Goal

Allow `GET /v1/videos/:task_id/content` to fetch video content from an administrator-configured internal channel base URL such as `http://cpa:8317`, while preserving SSRF protection for task-provided and externally derived video URLs.

## Current Failure

`VideoProxy` marks OpenAI, Sora, and Seedance channel content URLs with `skipFetchURLValidation`, but it still sends the request through `GetSSRFProtectedHTTPClient`. The protected transport validates the URL again during `RoundTrip` and rejects the internal CPA port with `port 8317 is not allowed`.

## Design

Determine whether the request will use a channel content URL before selecting the HTTP client.

- Channel content URLs constructed from the administrator-controlled channel `baseURL` use `service.GetHttpClient`.
- When the channel has an explicit proxy, retain `service.GetHttpClientWithProxy`.
- Task result URLs and other externally derived URLs continue to use `service.GetSSRFProtectedHTTPClient` and the existing explicit validation.
- Existing redirect handling remains unchanged. Redirect targets continue to pass through the current redirect validation, including Seedance credential stripping and redirect limits.
- Data URLs do not require an HTTP client and retain the existing range behavior.

## Scope

Change only `controller/video_proxy.go` and its focused tests. Do not modify global SSRF settings, allowed ports, private-IP policy, CPA configuration, nginx, or deployment configuration.

## Tests

- Verify an OpenAI-compatible channel content URL on a private test server and non-default port is fetched successfully.
- Verify an unsupported channel result URL still uses SSRF protection and remains blocked when it targets a disallowed private endpoint.
- Run the focused controller tests and relevant service tests.

## Success Criteria

- Internal CPA video content is fetched through New API and streamed from the platform `/content` endpoint.
- Untrusted video URLs retain the existing SSRF protections.
- Existing privacy, range, redirect, and credential-filtering tests remain green.

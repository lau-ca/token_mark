# xAI Image Aspect Ratio Design

## Goal

Forward the client-provided `aspect_ratio` field to xAI image-generation upstreams so Grok can generate non-square images.

## Design

Keep the public `relaykit/dto.ImageRequest` unchanged. Its existing `Extra` map already captures provider-specific request fields. The xAI adaptor will decode `Extra["aspect_ratio"]` as a string and copy a non-empty value into the xAI request DTO.

Missing, empty, or non-string values remain omitted. This preserves current behavior and leaves validation to xAI, whose supported values include `16:9`, `9:16`, `4:3`, and `3:4`.

## Scope

- Add `aspect_ratio` to the xAI image request DTO.
- Forward a valid string from the normalized request's `Extra` map.
- Add deterministic adaptor tests for supported ratios and omission behavior.
- Do not change other providers, the shared relaykit API, frontend code, or channel configuration.

## Verification

Run the xAI channel package tests with `go test ./relay/channel/xai`.

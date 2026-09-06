# Image Response URL Prefix Design

## Goal

Allow an administrator to configure one trusted image-response URL prefix per channel while preserving the existing hide-URL switch.

## Behavior

- When `force_image_b64_json_no_url` is enabled, remove `url` and `_provider_image_url` exactly as today.
- When it is disabled and `image_response_url_prefix` is empty, return upstream image URLs unchanged.
- When it is disabled and the prefix is configured, every returned `url` and `_provider_image_url` string must start with that prefix.
- If any checked URL does not match, stop the response and return HTTP 502 with the message `openai error.`
- Apply the rule to image generations and edits, including normal JSON, JSON converted to SSE, and native SSE responses.

## Configuration

Store the trimmed prefix in the channel `settings` JSON as `image_response_url_prefix`. Show one input under the existing hide-image-URL switch only while the switch is disabled. An empty value means no restriction.

## Validation

The prefix is a literal case-sensitive string prefix. The backend trims surrounding whitespace when validating and using it. No URL rewriting or hostname interpretation is performed.

## Tests

Cover the three configuration states, both protected response fields, multiple images, and generation/edit response paths. Verify a mismatch produces status 502 and the exact message.

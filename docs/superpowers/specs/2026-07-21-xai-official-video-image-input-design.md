# xAI Official Video Image Input Design

## Goal

Accept the official xAI image-to-video `image` object in New API without changing the request protocol or upstream payload of Seedance and other task channels.

## Scope

This change covers the New API xAI task adaptor for:

- `grok-imagine-video`
- `grok-imagine-video-1.5-preview`

It does not modify the existing Seedance adaptor, the planned external Seedance provider configuration, or CLIProxyAPI Grok 4.5 translation behavior.

The Grok 4.5 translator gap remains tracked in `router-for-me/CLIProxyAPI#4464`. The current repository permission is `READ`, and CLIProxyAPI repository instructions prohibit a standalone translator modification without write permission.

## Official Input Contract

The xAI REST API accepts one image object containing exactly one of:

```json
{"image":{"url":"https://example.com/frame.png"}}
```

```json
{"image":{"url":"data:image/png;base64,..."}}
```

```json
{"image":{"file_id":"file_xxx"}}
```

New API will continue accepting the existing compatibility form:

```json
{"image":"https://example.com/frame.png"}
```

Multipart image uploads, `input_reference`, and the existing string `images` array remain supported.

## Architecture

Provider wire formats must be parsed by their task adaptor. The shared `TaskSubmitReq` remains the canonical internal request and keeps `Image string` so existing task channels do not change.

The common task validation will be split into two responsibilities:

1. `ValidateMultipartDirect` parses the existing shared request format.
2. A reusable parsed-request validator validates and stores an already normalized `TaskSubmitReq`.

All existing channels continue calling `ValidateMultipartDirect`. The xAI adaptor uses an xAI-specific JSON parser and then calls the parsed-request validator. Multipart xAI requests continue using `ValidateMultipartDirect`.

## xAI JSON Parsing

The xAI parser will:

1. Decode the JSON body into raw fields.
2. Remove and parse `image` independently.
3. Decode the remaining fields into `TaskSubmitReq`, preserving duration, seconds, size, aspect ratio, ratio, resolution, and legacy image fields.
4. Normalize a string image or `image.url` into the canonical `TaskSubmitReq.Image` URL.
5. Store `image.file_id` as xAI adaptor-local context data instead of adding it to the shared task DTO.
6. Reject an image object that contains both `url` and `file_id`, neither field, or a non-string field value.
7. Reject ambiguous requests that combine the official `image` object with another image source.

After common validation succeeds, `image.file_id` marks the task as image-to-video.

## Upstream Request

The xAI upstream payload will continue using the official object shape:

```json
{
  "model": "grok-imagine-video-1.5-preview",
  "prompt": "animate",
  "duration": 7,
  "aspect_ratio": "9:16",
  "resolution": "720p",
  "image": {"url": "https://example.com/frame.png"}
}
```

For Files API input it will emit:

```json
{"image":{"file_id":"file_xxx"}}
```

The current aspect ratio, resolution, and duration forwarding behavior remains unchanged.

## Seedance Isolation

Seedance continues using the existing shared JSON parser and `referenceImages` contract. It will not accept xAI's `image` object through this change, and its request body construction remains untouched.

No changes are made to:

- `relay/channel/task/seedance`
- Seedance task routing
- Seedance billing
- Seedance result URL refresh
- Seedance content proxying

## Error Handling

Malformed xAI image input returns a local HTTP 400 error before billing or upstream submission. Error messages identify whether `image` must be a string or object and whether exactly one of `url` or `file_id` is required.

Upstream download or file ownership errors remain upstream xAI errors and are not converted into local validation errors.

## Tests

Focused xAI tests will cover:

- official public URL object;
- official base64 data URI object;
- official `file_id` object;
- legacy string image input;
- multipart input regression;
- conflicting `url` and `file_id`;
- empty image object;
- official object combined with a legacy image source;
- existing duration, aspect ratio, and resolution forwarding.

Seedance regression tests will confirm its existing `referenceImages` request body remains unchanged and that the xAI official object is not silently accepted and discarded.

Run focused tests for `relay/channel/task/xai`, `relay/channel/task/seedance`, and `relay/common`, followed by compilation of all task adaptors.

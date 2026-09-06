# Channel Request Retries Design

## Goal

Allow an administrator to configure how many times a failed request should be retried on the same channel. The setting applies to every model published by that channel.

## Configuration

The channel editor exposes an integer `retry_times` value from zero through three:

- `0` disables same-channel retries and is the default for existing channels.
- A positive value is the number of additional attempts after the initial request.
- The value is stored in the existing channel `setting` JSON, so no database migration is required.

## Relay behavior

The normal synchronous relay path first uses the selected channel, then repeats that same channel up to `retry_times` whenever the attempt does not succeed. Same-channel retries do not consult the automatic retry status-code configuration or the error's skip-retry marker. After those attempts are exhausted, the existing global retry loop may select another channel exactly as it does today and continues to use its existing status-code and skip-retry rules.

A same-channel retry is allowed only when retries remain, an error exists, the response has not been written, and the client request context is still active. Realtime and task relay paths do not use this setting. Multi-key channels reuse the existing channel setup path so another enabled key may be selected for a retry.

## Compatibility and risk controls

Missing settings decode as zero, so unchanged channels preserve current behavior. Save-time validation and runtime clamping both enforce the maximum of three. Requests that may already have reached the upstream can incur duplicate upstream work or charges, including failures such as HTTP 400. Administrators explicitly opt into that behavior by setting a positive channel retry count.

## Verification

Backend tests cover JSON compatibility, bounds validation, runtime clamping, HTTP 400 errors, skip-retry-marked errors, written responses, and canceled request contexts. Frontend tests cover legacy defaults, serialization, and administrator bounds. The independently buildable `relaykit` module must continue to pass with `GOWORK=off`.

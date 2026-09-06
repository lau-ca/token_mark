# Model Success Rate API Implementation Plan

## Task 1: Add portable log aggregation

- Add `model/model_success_rate.go` with a window aggregate type and exact-model query.
- Count distinct successful `request_id` values.
- Count only the latest unconsumed error per failed request and classify 4xx, 5xx, and other status codes.
- Parse the selected final errors with the project JSON wrapper so the same code works on SQLite, MySQL, and PostgreSQL.
- Add deterministic SQLite tests for deduplication, consume-over-error behavior, latest-error classification, exact model matching, and time boundaries.
- Run `go test ./model -run ModelSuccessRate`.

## Task 2: Add service response and cache

- Add `service/model_success_rate.go` with the public response DTOs.
- Calculate Beijing-day and rolling-five-minute windows from one timestamp.
- Calculate the two nullable error-rate percentages rounded to four decimals, including and excluding 4xx errors.
- Add a 15-second Redis cache keyed by a hash of the exact model identifier.
- Add service tests for window boundaries, rate formulas, and empty-window null rates.
- Run `go test ./service -run ModelSuccessRate`.

## Task 3: Add authenticated HTTP route

- Add `controller/model_success_rate.go` with trimmed non-empty model validation and safe error responses.
- Register `GET /api/model/success-rate` with `middleware.UserAuth()` in `router/api-router.go`.
- Add controller/router tests for missing model and the authenticated route contract.
- Run focused controller/router tests.

## Task 4: Verify the scoped implementation

- Run `gofmt` on new and modified Go files.
- Run all focused tests together.
- Run `go test ./model ./service ./controller ./router` if focused tests pass.
- Review `git diff` and confirm only intended lines were added to the already-dirty router file.

## Task 5: Deploy master only

- Build the current workspace snapshot for Linux AMD64 without modifying worker configuration.
- Create a timestamped backup of the current master image and Compose file on `148.113.178.75`.
- Upload the image, verify its checksum, and load it manually.
- Recreate only the `new-api-master` Compose service.
- Verify master health, restart count, `/api/status`, unauthenticated route behavior, and that the worker container ID is unchanged.

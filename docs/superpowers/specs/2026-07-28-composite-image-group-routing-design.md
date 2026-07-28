# Composite Image Group Routing Design

## Goal

Allow an administrator to create a composite token group that exposes one administrator-defined request model and routes image generation and image editing requests through an ordered list of existing internal models.

The urgent configuration is:

```text
composite token group
└── administrator-defined public request model
    ├── priority 1: gpt-image-2-w in its existing physical group
    └── priority 2: gpt-image-2 in its existing physical group
```

The design must also allow later route targets backed by Gemini or other image providers without adding provider-specific branching to the composite router.

## Confirmed Product Behavior

The user opts in by selecting a composite group when creating an API token. The administrator defines which model identifier clients must send when using that group.

Example:

```text
Composite group: image_stable
Public request model: gpt-image-2
```

The client then sends:

```json
{
  "model": "gpt-image-2"
}
```

The public request model may have the same name as an existing physical model. This is safe because the token group, not the model name alone, determines whether composite routing is used.

## Non-Negotiable Isolation

Existing model calls must retain their current routing, retry, billing, pricing, subscription, logging, and response behavior.

The runtime boundary is:

```text
token group is an enabled composite group?
├── no  -> execute the existing relay flow unchanged
└── yes -> validate the configured public request model and execute composite routing
```

The composite lookup must not mutate request context when the token group is not composite. Existing physical groups, channel model lists, model mappings, `AutoGroups`, group ratios, and global retry settings are not migrated or rewritten.

For an enabled composite group:

- a request using its configured public model enters composite routing;
- any other requested model fails closed with a clear model-not-allowed error;
- it never falls through to ordinary physical-group routing;
- an invalid or disabled composite configuration affects only tokens selecting that composite group.

Therefore these existing calls remain unchanged:

```text
ordinary token group + gpt-image-2-w -> existing flow
ordinary token group + gpt-image-2   -> existing flow
```

Only this new path is added:

```text
composite token group + configured public model -> composite flow
```

## Terminology

- **Composite group**: a virtual token group selected during API-token creation.
- **Public request model**: the administrator-defined model ID clients send for that composite group.
- **Operation**: image generation or image editing.
- **Route target**: one ordered destination containing a physical group and internal model.
- **Physical group**: an existing group already used by channels and ordinary tokens.
- **Internal model**: an existing model whose current billing configuration is reused.
- **Upstream model**: the final model sent to a selected channel after existing channel model mapping.
- **Initial attempt**: the first call to a route target.
- **Retry count**: additional calls allowed after the initial attempt.

The request keeps these identities separate:

```text
TokenGroupName     = image_stable
PublicModelName    = gpt-image-2
PhysicalGroupName  = gpt_image_web
BillingModelName   = gpt-image-2-w
UpstreamModelName  = gpt-image-2
```

## First-Version Scope

The first version supports only:

- `POST /v1/images/generations`;
- `POST /v1/images/edits` and the existing equivalent edit route aliases handled by the image relay.

Text, audio, video, embeddings, chat, Responses, and asynchronous task routing are outside the first version. A composite group rejects unsupported endpoints before selecting a channel or reserving quota.

This scope limits regression risk while establishing reusable image-operation boundaries for future Gemini image targets.

## Configuration Model

Composite groups are stored independently from existing physical groups and pricing settings.

### Composite group

- stable numeric primary key;
- unique composite group identifier stored in `Token.Group`;
- administrator-defined public request model;
- display name and description shown during token creation;
- enabled flag;
- user-selectable flag;
- pricing-page visibility flag;
- supported image operations;
- created and updated timestamps.

Composite group identifiers cannot conflict with physical group identifiers. Public request models may conflict with physical model identifiers because token-group dispatch provides isolation.

### Operation policy

Each composite group contains a policy for each enabled operation:

- image generation;
- image editing.

Generation and editing may share the same ordered targets initially, but they are stored independently so future providers can use different route orders or retry settings.

### Route target

- composite group ID;
- operation;
- route order;
- existing physical group identifier;
- existing internal model identifier;
- retry count;
- retryable status-code policy;
- enabled flag.

The first version does not add weights between route targets, nested composite groups, conditional expressions, per-route timeout overrides, or provider-specific request transformations.

## Administration UI

Add an administrator-only section at:

```text
/system-settings/models/composite-groups
```

Navigation location:

```text
System Settings
└── Models & Routing
    └── Composite Groups
```

The page provides:

- composite-group list and enabled state;
- public request model configuration;
- token-creation display name and description;
- user-selectable setting;
- separate generation and editing route lists;
- sortable route targets;
- physical-group and internal-model selectors;
- per-target retry count;
- retryable status-code configuration;
- read-only display of each internal model's current billing mode;
- validation results before enablement;
- enable, disable, and delete actions.

The channel page continues to show only physical groups and physical models. It may show a read-only reference indicating which composite groups use a channel's group and model, but composite public models are never inserted into a channel's `Models` field or ability cache.

## API-Token Creation

Enabled and user-selectable composite groups are added to the existing group selector alongside ordinary groups.

Selecting a composite group stores its identifier in the existing `Token.Group` field. It does not enable global `auto` grouping and does not change `CrossGroupRetry`.

The selector clearly labels composite groups so users understand that the group has administrator-defined routing. Existing ordinary groups and existing tokens remain unchanged.

If API-token model limits are enabled, the token permits the composite group's public request model. Internal models do not need to be exposed to the user or added to that token's model limit.

The group-list API returns an additive union of currently usable physical groups and enabled user-selectable composite groups. Composite groups are not inserted into `GroupRatio` merely to make them visible.

During token authentication, the token group is resolved in this order:

1. if it is an enabled composite group, validate composite-group availability and preserve it as the token group;
2. otherwise, execute the existing physical-group authorization and `GroupRatio` checks unchanged.

This ordering is required because the current physical-group authentication path rejects group names that are absent from `GroupRatio`. Composite groups use the selected physical target's ratio at call time and therefore do not need their own synthetic ratio.

## Configuration Validation

A composite group can be enabled only when:

- its composite group identifier is non-empty and does not conflict with a physical group or another composite group;
- its public request model is non-empty;
- at least one image operation is enabled;
- every enabled operation has at least one enabled route target;
- route order values are unique and normalized;
- every referenced physical group exists;
- every internal model has valid billing configuration;
- the physical group has an enabled channel capable of serving the internal model for that operation;
- retry counts are within a configured safety bound;
- retry status-code expressions are valid;
- no route target refers to a composite group;
- the configured targets accept compatible request semantics for the operation.

Configuration updates are validated completely and published atomically. Readers observe either the previous valid version or the new valid version.

## Runtime Dispatch

Composite dispatch occurs before ordinary physical-group channel selection.

The current distributor selects a channel before the relay controller calculates pricing. The implementation therefore adds one gated distributor branch:

```text
composite token group
-> validate public model and image endpoint
-> preserve the composite policy in request context
-> skip ordinary initial channel selection
-> continue to the composite image coordinator
```

The ordinary distributor branch remains unchanged. A physical channel context is installed only after the composite coordinator selects a route target.

### Ordinary token group

If `Token.Group` is not an enabled composite group, the current distributor, pricing, retry, relay, and settlement flow runs without composite context or model mutation.

### Composite token group

For a matching composite group:

1. Preserve the administrator-defined public request model as the user-visible model.
2. Reject requests whose model does not exactly match the group's configured public request model.
3. Classify the endpoint as image generation or image editing.
4. Reject unsupported operations before billing or channel selection.
5. Load one immutable composite-policy snapshot for the request.
6. Iterate route targets in configured order for that operation.
7. Resolve channels using the target's physical group and internal model through existing priority and weight logic.
8. Set the billing identity to the internal model.
9. Apply existing channel model mapping to obtain the upstream model.
10. Execute the initial attempt and configured retries.
11. Advance to the next target only after a retryable failure exhausts the current target.
12. Stop immediately on success or a non-retryable error.

The composite flow does not use global `AutoGroups` or global cross-group retry. Its order and retry counts are local to one composite group and one image operation.

The relay controller dispatches to the composite image coordinator only when the distributor stored a validated composite policy. Otherwise it executes the existing relay controller body unchanged.

## Attempt and Retry Semantics

Retry count means additional attempts after the initial call:

```text
retry count 0 -> at most 1 total attempt
retry count 1 -> at most 2 total attempts
retry count 2 -> at most 3 total attempts
```

Example:

```text
target A retry count 2
target B retry count 1

A initial -> A retry 1 -> A retry 2 -> B initial -> B retry 1
```

Each retry reselects an eligible channel inside the same physical group and internal model using existing channel priority, weight, exclusion, and availability rules.

## Success Definition

The selected channel adapter and the existing image relay remain authoritative for protocol success. The composite coordinator does not reinterpret successful responses from ordinary channels.

An image attempt succeeds when all applicable conditions are true:

- the upstream transport completes without error;
- the upstream returns a successful protocol status;
- the selected channel adapter parses and writes the response successfully;
- the existing image relay returns no `NewAPIError`;
- the response can still be returned consistently to the client.

HTTP 2xx alone is not sufficient. If the selected adapter reports malformed content, an upstream error payload, or missing required image output as an error, the composite coordinator treats it as a failed attempt. The composite feature does not change adapter success semantics for ordinary model calls.

For streaming image responses, retry or target switching is permitted only before response headers or image events are committed to the client. Once any irreversible response data has been sent, the request cannot retry or switch targets.

## Error Classification

### Retryable by default

- DNS, TLS, connection, and transport failures;
- upstream connection or read timeout before client response commitment;
- channel unavailable or channel-classified failure;
- upstream rate limit `429`;
- configured upstream `5xx` responses;
- successful HTTP status with malformed or missing normalized image output;
- provider error payloads classified by the adapter as transient upstream failures.

### Non-retryable by default

- invalid client image parameters;
- unsupported size, format, quality, or edit-mask input;
- request-body or multipart parsing failure;
- model mismatch for the selected composite group;
- API-token permission or model-limit failure;
- insufficient quota or subscription eligibility failure;
- sensitive-content rejection classified as a client-policy error;
- request body too large;
- most client `400`, `401`, `403`, `404`, and `422` responses;
- any error after irreversible response commitment.

Administrators may configure retryable HTTP status codes per route target, but local validation, authentication, billing, and already-committed-response errors can never be made retryable by configuration.

## Billing Lifecycle

The public request model controls the client contract. The current route target's internal model controls billing.

Before the first attempt of each route target:

1. Resolve the internal model's existing fixed-price, token-ratio, or expression billing configuration.
2. Resolve the physical target group's existing billing ratio.
3. Build an immutable pricing snapshot.
4. Calculate the required reservation.
5. Reserve additional quota before sending the target request.
6. If reservation fails, do not call that target.

Moving from `gpt-image-2-w` fixed-price billing to `gpt-image-2` token billing reprices the request before the fallback attempt. Existing `BillingSession.Reserve` behavior is reused for safe additional reservation.

Only the successful target's internal-model pricing snapshot is used for settlement. If every target fails, the existing refund and violation-fee behavior applies.

Subscription and API-token permission checks use the public request model. Quota calculation and administrative billing evidence use the successful internal model and physical group.

The composite coordinator creates one billing session for the request. Before every new route target it replaces the target pricing snapshot and calls `BillingSession.Reserve` when the new reservation is higher. A lower target reservation is reconciled by final settlement rather than by reducing the live reservation before the attempt.

The existing image relay settles only after a successful adapter response through `PostTextConsumeQuota`. Failed attempts return an error before settlement, allowing the coordinator to retry or advance without recording a successful consume log.

## Pricing and Model Discovery

The composite group is selected during token creation; it is not presented as a physical channel group.

The administrator-defined public request model may already exist on `/pricing`. The composite feature does not replace or globally alter that model's physical pricing entry.

When the pricing page needs to explain composite-group behavior, it may show group-specific routing and billing options only when that composite group is selected. Existing pricing payloads and displays for ordinary groups remain unchanged.

## Logs and Metrics

User-facing logs show:

- the administrator-defined public request model;
- the selected composite group.

Administrator-only metadata also records:

- composite group ID;
- operation;
- selected route order;
- internal billing model;
- physical group and channel;
- initial-attempt or retry number;
- pricing mode and snapshot;
- normalized failure class for each failed attempt.

Existing ordinary model metrics remain unchanged. Composite requests can aggregate under the public model and composite group while retaining internal route dimensions for diagnosis.

## Gemini and Future Image Providers

The composite router contains no GPT- or Gemini-specific branches.

Existing provider adapters remain responsible for:

- translating normalized image generation or editing requests;
- applying provider-specific model mapping;
- parsing provider responses;
- producing the existing normalized image result or normalized error.

The composite router consumes only normalized operation, success, and error outcomes. Adding a future Gemini image route therefore consists of:

1. ensuring the Gemini channel adapter supports the required image operation;
2. configuring its physical group and internal model as a route target;
3. validating billing and response normalization.

No composite-routing code change should be required.

## Caching and Concurrency

Enabled composite-group definitions are cached as immutable snapshots. A successful update atomically replaces the entire snapshot.

Requests in progress keep the version loaded at request start. New requests use the latest published version. Refresh failure retains the last valid snapshot instead of publishing empty or partial configuration.

## Failure Containment and Kill Switch

The feature has a global enable switch that defaults to disabled.

- Disabled: composite groups are not offered during token creation, existing composite-group tokens are rejected explicitly, and ordinary-group traffic follows the current path.
- Enabled with no definitions: ordinary traffic remains unchanged.
- Disabled or invalid composite group: only tokens selecting that group are rejected.
- Composite runtime failure: only that request fails; ordinary token groups continue through the existing flow.

Disabling one composite group or the global feature takes effect through atomic configuration refresh and requires no code rollback.

## Database Compatibility

Use additive tables and GORM migrations compatible with SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+.

Do not alter existing channel, model, token, log, pricing, or group-ratio columns in the first version. The existing `Token.Group` field stores the composite group identifier. Service validation maintains references without relying on database-specific cascade behavior.

## Testing and Acceptance Criteria

### Isolation

- Feature disabled: all existing model and group calls preserve current behavior.
- Feature enabled with no matching composite group: all existing calls preserve current behavior.
- Ordinary groups calling `gpt-image-2-w` remain unchanged.
- Ordinary groups calling `gpt-image-2` remain unchanged.
- A public model name shared by physical and composite paths is dispatched solely by token group.
- Composite public models are never inserted into physical channel abilities.
- Invalid composite configuration affects only the corresponding composite group.

### Token and model contract

- Enabled selectable composite groups appear during token creation.
- The configured public request model is accepted for that composite group.
- Any other requested model is rejected without physical-group fallback.
- API-token model limits validate the public request model, not internal models.

### Image operations

- Generation and editing policies are selected independently.
- Unsupported endpoints are rejected before billing and channel selection.
- A valid image URL response succeeds.
- A valid base64 image response succeeds.
- HTTP 2xx with no image result is treated as an upstream failure.
- Multipart edit bodies can be replayed safely before each attempt.
- Streaming fallback stops after irreversible response commitment.

### Priority and retries

- The first target succeeds without touching later targets.
- Retry count zero performs one total attempt.
- Same-target retries stop at the configured count.
- Retryable exhaustion advances to the next target.
- Non-retryable client failure stops immediately.
- Each retry preserves existing physical-group priority, weight, and channel exclusion behavior.

### Billing

- Fixed-price target success settles with the fixed-price snapshot.
- Token-priced target success settles with the token snapshot.
- Fixed-price failure followed by token success reprices before fallback.
- Insufficient fallback reservation prevents the fallback upstream request.
- All-target failure refunds correctly.
- Wallet, subscription, trusted quota, unlimited token, and saturation paths preserve current invariants.

### UI and API

- Only administrators manage composite groups.
- Validation errors identify the operation, target, and field.
- Existing group-pricing and model-pricing payloads remain compatible.
- Existing channel pages continue to show physical data only.
- User logs show public model and composite group; administrator metadata shows internal routing.

## Deployment Boundary

Composite dispatch and route-specific billing execute on every instance serving relay traffic. Code and additive migrations are deployed with the global switch disabled.

Safe rollout order:

1. deploy the disabled feature and migration;
2. verify ordinary GPT image generation and editing behavior;
3. update relay-serving workers only after explicit user authorization;
4. create a disabled composite group and validate it;
5. enable it for controlled tokens;
6. verify generation, editing, retry, fallback, and billing;
7. make the composite group generally selectable.

Master-only deployment is insufficient when workers serve relay requests. Worker deployment remains subject to explicit user instruction.

# Composite Model Routing Design

## Goal

Provide an administrator-configured composite model that exposes one stable public model identifier while routing each request through an ordered list of existing group and internal-model targets.

The feature must be protocol-agnostic. The initial use case combines `gpt-image-2-w` and `gpt-image-2`, but the same configuration and runtime must support future Gemini image models, text models, and task models without provider-specific routing code.

## Non-Negotiable Isolation

Existing models must retain their current routing, billing, pricing, subscription, logging, and API behavior.

The runtime boundary is:

```text
requested model has an enabled composite definition?
├── no  -> execute the existing relay flow unchanged
└── yes -> execute the composite routing flow
```

The composite resolver must return `not found` without mutating the request context. Existing model and group settings are not migrated into the new configuration. An invalid composite definition may fail only that composite model; it must never fall through to an ordinary model or change global routing behavior.

## Terminology

- **Public model**: the model identifier sent by the user, for example `gpt-image-2-stable`.
- **Route target**: one ordered destination in a composite model, consisting of a group and an internal model.
- **Internal model**: an existing billable model, for example `gpt-image-2-w` or `gpt-image-2`.
- **Upstream model**: the model name ultimately sent to the selected channel after existing channel model mapping.
- **Attempt**: one upstream request sent through a selected channel.

These identities remain separate throughout the request:

```text
PublicModelName   = gpt-image-2-stable
BillingModelName  = gpt-image-2-w
UpstreamModelName = gpt-image-2
```

## Configuration Model

Composite definitions are stored independently from the existing model, channel, group-ratio, and `AutoGroups` settings.

### Composite model

- stable numeric primary key;
- unique public model identifier;
- display name and description;
- enabled flag;
- pricing-page visibility flag;
- supported endpoint types;
- ordered route targets;
- created and updated timestamps.

### Route target

- composite model ID;
- route order;
- existing group identifier;
- existing internal model identifier;
- maximum attempts;
- retry status-code policy;
- enabled flag.

The first version does not add per-route timeout overrides, weighted routing, nested composite models, conditional routing expressions, or provider-specific request transformations.

Composite models cannot reference another composite model. This prevents cycles and keeps billing and failure handling auditable.

## Administration UI

Add an administrator-only section at:

```text
/system-settings/models/composite-models
```

Navigation location:

```text
System Settings
└── Models & Routing
    └── Composite Models
```

The page provides:

- a list of composite models and their enabled state;
- creation and editing in a drawer;
- a sortable route-target list;
- group and internal-model selectors backed by existing data;
- per-target maximum attempts and retry-code configuration;
- validation results before enablement;
- a read-only summary of the referenced internal model's billing mode;
- enable, disable, and delete actions.

The channel page continues to show only physical channel groups and models. It may show a read-only “used by composite models” reference, but it must not add the public composite model to a channel's `Models` field or ability cache.

## Validation

A composite definition can be enabled only when:

- its public model identifier is non-empty and does not conflict with an existing physical model or another composite model;
- it contains at least one enabled route target;
- route order values are unique and contiguous after normalization;
- every referenced group exists;
- every internal model exists and has valid billing configuration;
- the referenced group has at least one enabled channel capable of serving the internal model for each declared endpoint type;
- maximum attempts are within the configured safety bound;
- retry status-code expressions are valid;
- the internal model is not another composite model;
- all route targets support compatible request and response semantics for the declared endpoint types.

Configuration updates are validated completely and published atomically. Readers observe either the previous valid version or the new valid version, never a partially updated route list.

## Runtime Routing

### Existing model request

If no enabled composite definition matches the requested model, the controller calls the existing relay path without changing model names, groups, retry counters, pricing data, or request context.

### Composite model request

For a matching composite model:

1. Preserve the requested public model in `OriginModelName`.
2. Validate the endpoint against the composite model's supported endpoint types.
3. Load one immutable snapshot of the composite definition for the request.
4. Iterate enabled route targets in configured order.
5. For each target, resolve channels using the existing group, internal model, priority, and weight logic.
6. Set `BillingModelName` to the target's internal model.
7. Apply existing channel model mapping to derive `UpstreamModelName`.
8. Send no more than the target's configured maximum attempts.
9. Move to the next target only when the error is retryable under that target's policy.
10. Stop immediately on success or a non-retryable client error.

The composite flow does not use global `AutoGroups`. Route order and attempts are local to the composite definition, so changing one composite model cannot change another model's routing.

## Retry Semantics

Retry policy is evaluated per target.

The default recommended policy retries channel failures, transport failures, timeouts, rate limits, and configured upstream server errors. Client request errors are not retried by default.

A route target's maximum attempts includes the first attempt. A value of `1` means no same-target retry. Exhausting a target permits advancing to the next target only when the last error is retryable.

The final error response follows the existing error normalization and sensitive-data masking rules. Administrative logs retain the attempt history.

## Billing Lifecycle

Billing is determined by the internal model of the route that is about to be attempted, not by the public composite model.

Before the first attempt of a target:

1. Resolve the internal model's existing fixed-price, token-ratio, or expression billing configuration.
2. Build an immutable pricing snapshot for that target.
3. Calculate the target reservation.
4. Reserve any additional quota before sending the upstream request.
5. If reservation fails, do not send that target's request.

When moving from a fixed-price target to a token-priced target, the target is repriced before the fallback request. Existing `BillingSession.Reserve` behavior is reused to increase the reservation safely. If final settlement is lower than the reservation, the normal settlement path returns the difference.

Only the successful target's pricing snapshot is used for final settlement. When every target fails, the normal refund and violation-fee rules apply.

Subscription eligibility is checked against the public composite model, while quota calculation uses the selected internal billing model. Subscription plans can therefore explicitly allow or deny the public model without exposing internal model identifiers to users.

## Pricing and Model Discovery

The public composite model appears as one model in model discovery, API-key model limits, and `/pricing` when visibility is enabled.

Existing pricing response fields remain unchanged for physical models. Composite models add an optional `billing_options` field describing their possible billing outcomes. Existing clients that ignore unknown fields remain compatible.

The pricing UI explains that the final charge follows the successful route. Internal models may remain visible or hidden independently according to existing metadata settings.

## Logs and Metrics

User-facing logs and responses show the public model identifier.

Administrative log metadata records:

- composite model ID and public model identifier;
- selected route order;
- internal billing model;
- selected group and channel;
- attempt number;
- pricing mode and pricing snapshot identifier;
- previous failed targets and normalized failure classes.

Existing model success-rate and performance metrics continue to aggregate physical models as they do today. Composite-model metrics can aggregate by public model while retaining internal route dimensions in administrator-only metadata.

## API Key Behavior

Composite models participate in existing API-key model limits as public model identifiers.

No new API-key smart-group switch is required for the first version. Selecting or allowing the composite model is the user's explicit opt-in to its routing policy. Keys that do not allow the composite model cannot call it. Existing keys and existing group selection remain unchanged.

## Caching and Concurrency

Enabled composite definitions are cached as immutable snapshots. Cache refresh replaces the full snapshot atomically after a successful database update.

Requests already in progress continue using the version loaded at request start. New requests use the newly published version. Cache failure falls back to the last valid snapshot; it does not publish an empty or partially decoded configuration.

## Failure Containment and Kill Switch

The feature has a global enable switch that defaults to disabled.

- Disabled: every request uses the existing path; configured composite model identifiers are unavailable.
- Enabled with no definitions: existing behavior remains unchanged.
- Invalid definition: only that composite model is unavailable.
- Runtime failure inside composite routing: return an error for that request and preserve existing model traffic.

Disabling a single composite model or the global switch takes effect through the atomic configuration refresh and does not require code rollback.

## Database Compatibility

Schema and queries must support SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+.

Use additive tables and GORM migrations. Do not alter existing channel, model, token, log, or pricing columns for the first version. Foreign-key behavior must not depend on database-specific cascade support; service validation protects references.

## Testing and Acceptance Criteria

### Isolation contract

- With the feature disabled, existing model routing and billing results are unchanged.
- With the feature enabled but no matching definition, existing model routing and billing results are unchanged.
- An invalid or disabled composite definition cannot alter a physical model request.
- Composite model identifiers are never inserted into physical channel ability records.

### Routing

- The first target succeeds without touching later targets.
- Same-target attempts stop at the configured maximum.
- Retryable failure advances to the next target.
- Non-retryable 4xx failure stops immediately.
- Group priority and weight selection are preserved inside each target.
- Different endpoint types reject incompatible target combinations before enablement.

### Billing

- Fixed-price target success settles with the fixed-price snapshot.
- Token-priced target success settles with the token snapshot.
- Fixed-price failure followed by token success reprices and reserves before fallback.
- Insufficient fallback reservation prevents the upstream fallback call.
- All-target failure refunds correctly.
- Wallet, subscription, trusted quota, unlimited token, and quota saturation paths preserve existing invariants.

### API and UI

- Only administrators can manage composite definitions.
- Configuration validation errors identify the specific target and field.
- Existing pricing payloads remain compatible.
- API-key model limits accept the public composite model.
- User logs show the public model; administrator metadata shows the internal route.

## Deployment Boundary

The feature changes relay routing and billing behavior. Any runtime instance that serves relay traffic must run the supporting code before the global switch is enabled.

Deployment is additive and disabled by default:

1. deploy code and database migration with the feature disabled;
2. verify existing model traffic and billing;
3. deploy every relay-serving worker only after explicit authorization;
4. create one disabled composite definition and validate it;
5. enable the definition for controlled testing;
6. enable public access only after billing and fallback verification.

Master-only deployment is insufficient when workers handle relay requests. Worker deployment remains subject to explicit user instruction.

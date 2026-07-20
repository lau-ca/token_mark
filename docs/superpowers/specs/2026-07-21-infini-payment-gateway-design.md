# Infini Payment Gateway Design

## Goal

Add Infini as an independent payment gateway for balance top-ups and one-time subscription-plan purchases without changing the behavior or configuration of Epay, Stripe, Creem, Waffo, or Waffo Pancake.

## Scope

- Add an Infini tab under `/system-settings/billing/payment`.
- Support production and sandbox Infini OpenAPI endpoints.
- Support balance top-ups through Infini Hosted Checkout.
- Support one-time purchases of existing local subscription plans through Infini Hosted Checkout.
- Verify Infini API requests with its HMAC-SHA256 request-signing protocol.
- Verify Infini Webhooks with the configured Webhook secret.
- Complete local orders only from a verified `order.completed` event whose payload status is `paid`.
- Treat `processing`, `partial_paid`, `expired`, and `late_payment` as non-success states. Expired local orders may be marked expired; partial or late payments remain auditable and require manual handling.

Automatic recurring billing through Infini subscriptions is outside this scope. Existing local plans remain one-time purchases that create the project's existing `UserSubscription` snapshot.

## Isolation Boundary

Infini receives its own:

- setting variables and option-map entries;
- API client and signature implementation;
- top-up controller;
- subscription purchase controller;
- public Webhook route;
- authenticated top-up and subscription routes;
- frontend request functions and payment dispatch branch;
- payment-settings tab.

Existing gateway controllers, settings, constants, routes, callbacks, and request payloads keep their current behavior. Shared order models and existing idempotent settlement functions are reused only through new `PaymentProviderInfini` checks.

## Configuration

The payment settings page exposes:

- `InfiniEnabled`
- `InfiniSandbox`
- `InfiniKeyId`
- `InfiniSecretKey`
- `InfiniWebhookSecret`
- `InfiniCurrency`
- `InfiniPayMethods`
- `InfiniMinTopUp`

Secrets are write-only in the form: existing values are not returned to the browser, and blank submissions preserve the stored values. Production uses `https://openapi.infini.money`; sandbox uses `https://openapi-sandbox.infini.money`.

`InfiniPayMethods` is a JSON integer array using Infini's documented values. The default is `[1]` so the gateway starts with crypto checkout unless the operator explicitly enables other methods available to the merchant account.

## Order Creation

### Balance top-up

1. Validate payment compliance, gateway availability, minimum amount, and calculated fiat charge.
2. Insert a local pending `TopUp` with `PaymentMethod=infini` and `PaymentProvider=infini`.
3. Call `POST /v1/acquiring/order` with a UUID `request_id`, the local trade number as `client_reference`, amount, currency, checkout methods, and return URLs.
4. If Infini rejects order creation, mark the local pending order failed.
5. Return `checkout_url` to the frontend and open it in a new browser tab.

### Subscription-plan purchase

1. Reuse existing plan validation and per-user purchase-limit rules.
2. Insert a pending `SubscriptionOrder` with provider `infini`.
3. Create an Infini one-time acquiring order using the plan price and local trade number.
4. Return `checkout_url`; do not use Infini recurring-subscription APIs.

## Webhook Processing

The public Webhook endpoint reads the raw JSON body once and requires:

- `X-Webhook-Timestamp`
- `X-Webhook-Event-Id`
- `X-Webhook-Signature`

It rejects stale timestamps, computes HMAC-SHA256 over `{timestamp}.{event_id}.{raw_body}`, and compares signatures in constant time. The local order is selected by `client_reference`, not trusted Infini display fields.

For `order.completed` with `status=paid`:

- a top-up calls a new provider-specific transactional recharge function guarded by `PaymentProviderInfini`;
- a subscription calls the existing `CompleteSubscriptionOrder` with `PaymentProviderInfini`;
- repeated delivery returns success without granting quota or a subscription twice.

Other valid events return success after logging their state so Infini does not retry indefinitely. An invalid signature, unknown local order, provider mismatch, or malformed payload does not settle anything.

## Frontend Behavior

Infini appears as a dedicated payment method when enabled. Selecting it uses the existing recharge amount UI, calls the Infini amount and pay endpoints, and opens the returned Hosted Checkout URL in a new tab. It does not use the Epay hidden-form submission path.

The settings page uses the project's existing shadcn form, tabs, inputs, switches, and JSON editor patterns. All new user-facing strings are registered in every supported locale.

## Tests

Backend tests cover:

- deterministic request signing and digest generation;
- production/sandbox base URL selection;
- Webhook signature verification, timestamp rejection, and malformed headers;
- `order.completed` settlement and duplicate-event idempotency;
- provider mismatch protection;
- balance top-up and subscription order payload construction.

Frontend tests cover payment-type dispatch so Infini opens `checkout_url` and never enters the Epay form path. Typecheck, lint for changed frontend files, targeted Go tests, and a production frontend build are required. Browser end-to-end testing is not included.

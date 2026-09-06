# Infini Payment Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an isolated Infini Hosted Checkout gateway for balance top-ups and one-time subscription-plan purchases.

**Architecture:** A dedicated Infini service signs OpenAPI requests and verifies Webhooks. New controllers create local pending orders before requesting Hosted Checkout and settle only verified `order.completed` callbacks through provider-guarded, idempotent model functions. Existing payment gateways remain unchanged except for shared lists and dispatch points that add the new provider.

**Tech Stack:** Go 1.22, Gin, GORM, HMAC-SHA256, React 19, TypeScript, React Query, React Hook Form, Zod, shadcn/Base UI, i18next, Bun, testify.

---

### Task 1: Infini settings and option persistence

**Files:**
- Create: `setting/payment_infini.go`
- Modify: `model/option.go`
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/billing/section-registry.tsx`
- Modify: `web/default/src/features/system-settings/integrations/payment-settings-section.tsx`

- [ ] Add typed settings with defaults:

```go
var InfiniEnabled = false
var InfiniSandbox = true
var InfiniKeyID = ""
var InfiniSecretKey = ""
var InfiniWebhookSecret = ""
var InfiniCurrency = "USD"
var InfiniPayMethods = "[1]"
var InfiniMinTopUp = 1
```

- [ ] Register every setting in `common.OptionMap` and `updateOptionMap`; parse booleans/integers without changing existing cases.
- [ ] Extend `BillingSettings`, section defaults, Zod schema, dirty comparison, and save updates.
- [ ] Add an `Infini` tab using existing `FormField`, `Input`, `Switch`, and `Textarea` components. Blank secret fields must preserve stored secrets.
- [ ] Run `gofmt` and frontend typecheck.

### Task 2: Infini API signing client

**Files:**
- Create: `service/infini.go`
- Create: `service/infini_test.go`

- [ ] Write deterministic tests for base URL selection, signing string, Digest, HMAC Authorization header, create-order decoding, and Webhook verification.
- [ ] Implement `InfiniCreateOrderRequest`, `InfiniCreateOrderResponse`, and a small client using `http.Client` and `common.Marshal`/`common.DecodeJson`.
- [ ] Build the signing string exactly as `keyID + "\n" + METHOD + " " + path + "\n" + "date: " + date + "\n"`.
- [ ] Add `Digest: SHA-256=<base64>` for JSON bodies and constant-time Webhook signature comparison over `timestamp.eventID.rawBody`.
- [ ] Reject Webhook timestamps outside five minutes and return explicit errors for missing credentials or malformed responses.
- [ ] Run `go test ./service -run Infini -count=1`.

### Task 3: Provider-safe model settlement

**Files:**
- Modify: `model/topup.go`
- Create: `model/topup_infini_test.go`

- [ ] Add `PaymentMethodInfini` and `PaymentProviderInfini` constants.
- [ ] Add `RechargeInfini(referenceID, callerIP)` using `DB.Transaction`, `lockForUpdate`, provider/status guards, quota update, completion timestamp, and top-up log creation.
- [ ] Keep the amount semantics identical to Epay-style fiat top-ups: `TopUp.Amount * common.QuotaPerUnit` is credited.
- [ ] Test success, duplicate callback behavior, and provider mismatch with explicit database fixtures.
- [ ] Run targeted model tests.

### Task 4: Top-up controller and Webhook

**Files:**
- Create: `controller/topup_infini.go`
- Create: `controller/topup_infini_test.go`
- Modify: `controller/topup.go`
- Modify: `controller/payment_webhook_availability.go`
- Modify: `router/api-router.go`

- [ ] Add `isInfiniTopUpEnabled` requiring compliance, enabled flag, API credentials, Webhook secret, valid currency, and valid pay-method JSON.
- [ ] Add `/api/user/infini/amount` and `/api/user/infini/pay`; calculate price with existing top-up group ratio and discounts, then insert a provider-isolated pending order.
- [ ] Create an Infini order with UUID `request_id`, local trade number as `client_reference`, configured currency/pay methods, and success/failure URLs.
- [ ] Mark local order failed when remote order creation fails and return only `checkout_url`/order identifiers to the frontend.
- [ ] Add public `/api/infini/webhook`, read the limited raw body once, verify headers/signature/timestamp, dispatch by `client_reference`, and settle only `order.completed` plus `status=paid`.
- [ ] Add Infini to `GetTopUpInfo` without modifying existing provider entries.
- [ ] Test invalid signature, expired timestamp, ignored states, provider mismatch, top-up completion, and duplicate callback.

### Task 5: One-time subscription-plan purchase

**Files:**
- Create: `controller/subscription_payment_infini.go`
- Create: `controller/subscription_payment_infini_test.go`
- Modify: `router/api-router.go`

- [ ] Reuse existing plan enabled/minimum/purchase-limit checks.
- [ ] Insert a pending `SubscriptionOrder` with `PaymentProviderInfini` before calling Infini.
- [ ] Create a one-time acquiring order for `plan.PriceAmount`; do not call Infini recurring subscription endpoints.
- [ ] Prefix or inspect the local trade number so the shared Infini Webhook can distinguish subscription orders from top-ups.
- [ ] On verified paid completion, call `model.CompleteSubscriptionOrder(tradeNo, rawPayload, model.PaymentProviderInfini, model.PaymentMethodInfini)`.
- [ ] Test order payload, failure status update, and paid completion.

### Task 6: Wallet and subscription frontend dispatch

**Files:**
- Modify: `web/default/src/features/wallet/constants.ts`
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/api.ts`
- Modify: `web/default/src/features/wallet/hooks/use-payment.ts`
- Modify: `web/default/src/features/wallet/lib/payment.ts`
- Modify: `web/default/src/features/wallet/lib/ui.tsx`
- Modify: `web/default/src/features/subscriptions/api.ts`
- Modify the existing subscription purchase component that dispatches provider requests.
- Add or modify focused Vitest files beside the payment dispatch logic.

- [ ] Add `PAYMENT_TYPES.INFINI` and a dedicated checkout response type.
- [ ] Route amount and pay requests to `/api/user/infini/*` only when the selected type is `infini`.
- [ ] Open `checkout_url` in a new tab and never call `submitPaymentForm` for Infini.
- [ ] Add the subscription API call and provider branch for `/api/subscription/infini/pay`.
- [ ] Add an Infini icon fallback consistent with existing payment icons.
- [ ] Test dispatch and redirect behavior with deterministic mocks.

### Task 7: i18n and verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] Add translations for the Infini settings tab, credentials, sandbox, pay methods, currency, minimum top-up, and checkout messages.
- [ ] Run `bun run i18n:sync` if static key extraction requires it and inspect the diff so unrelated translations are not rewritten.
- [ ] Run `gofmt` on changed Go files.
- [ ] Run targeted Go tests, then `go test ./controller ./model ./service`.
- [ ] Run changed frontend tests, `bun run typecheck`, lint for changed frontend files, and `bun run build`.
- [ ] Inspect `git diff --check` and `git status --short`; keep unrelated existing worktree changes untouched.

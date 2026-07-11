# Billing Expression Display Design

## Goal

Fix structured billing displays for request-dependent image and video expressions across `/pricing` and `/usage-logs/common` without changing expression evaluation or charged quota.

## Scope

- Parse canonical image expressions that select a tier with `image_size_tier(param("size"))` and multiply a fixed unit price by normalized `param("n")`.
- Display image tiers such as `1K`, `2K`, and `4K` with their per-image unit prices.
- Display Seedance video tiers with per-request prices for `480p` and `720p`, and per-second prices for `1080p` and `4k`.
- Exclude non-billable fallback tiers such as `tier("invalid", -1)` from pricing tables and log breakdowns while retaining them in the stored and evaluated expression.
- Reuse the same parsed representation in public pricing, usage-log summaries, and usage-log details.

## Design

Extend the existing frontend billing-expression parser rather than altering production expressions. The parser remains intentionally narrow and accepts only canonical, structurally safe shapes.

The parsed tier unit model will distinguish token, request, second, and image units. A canonical quantity expression is recognized only when it contains one numeric unit-price factor and the standard normalized `param("n")` fallback shape. The displayed value is the unit price, not the total request charge.

Fallback branches are excluded when their parsed price is non-positive. The `invalid` label receives no special billing behavior; filtering follows the non-positive price invariant so other legitimate labels remain unaffected.

Both pricing and usage logs continue consuming `parseTiersFromExpr` and `getTierUnitPrice`, keeping the display rules centralized.

## Safety

- No backend evaluator, quota conversion, pre-consume, settlement, or refund behavior changes.
- Unsupported expression shapes continue falling back to raw-expression display.
- Stored expressions remain unchanged, including rejection fallback branches.
- Existing token, per-request, and per-second parsing behavior remains covered by regression tests.

## Validation

- Parser tests for canonical image-size and quantity expressions.
- Parser tests confirming negative fallback tiers are omitted.
- Dynamic pricing tests for image, request, and second unit formatting.
- Usage-log tests confirming matched image/video tiers produce structured summaries.
- Frontend typecheck, targeted lint, relevant unit tests, and production build.

## Deployment

Deploy manually and sequentially to active nodes only:

1. Update the Work service on `192.241.132.211`, then verify health and logs.
2. Update the Work service on `157.230.213.186`, then verify health and logs.
3. Update the master service on `157.230.213.186`, then verify `/pricing`, status, and logs.

The removed `159.89.150.206` node is excluded. Each service is backed up before replacement; no Git rollback operation or deployment script is used.

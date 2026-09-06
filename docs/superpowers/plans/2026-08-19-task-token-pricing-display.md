# Task Token Pricing Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render `task_tokens(...)` billing expressions as a complete, readable resolution and reference-video price matrix on the public pricing page.

**Architecture:** Extend the existing narrow billing-expression parser with a targeted `task_tokens` branch that emits the same `ParsedTier` shape already consumed by pricing cards, tables, model details, group pricing, and usage logs. Add task-specific metadata to those tiers so the detailed breakdown can render a two-column reference-video matrix while all other pricing surfaces share the same six parsed combinations and retain the raw-expression fallback for unsupported syntax.

**Tech Stack:** React 19, TypeScript, Bun test runner, i18next, Tailwind CSS.

---

### Task 1: Parse task-token price combinations

**Files:**
- Modify: `web/src/features/pricing/lib/billing-expr.ts`
- Test: `web/src/features/pricing/lib/__tests__/billing-expr.test.ts`

- [ ] Add a deterministic test covering all six Seedance resolution/reference-video prices.
- [ ] Add narrow parsing for the supported `task_tokens` conditional shape.
- [ ] Preserve the existing empty-result fallback for unsupported expressions.
- [ ] Run the focused parser test.

### Task 2: Render the complete pricing information

**Files:**
- Modify: `web/src/features/pricing/lib/dynamic-price.ts`
- Modify: `web/src/features/pricing/components/model-card.tsx`
- Modify: `web/src/features/pricing/components/pricing-columns.tsx`
- Modify: `web/src/features/pricing/components/model-details.tsx`
- Modify: `web/src/features/pricing/components/dynamic-pricing-breakdown.tsx`

- [ ] Mark parsed task-token summaries separately from unsupported special expressions.
- [ ] Show a concise combination count on cards and list rows.
- [ ] Render resolution, without-reference, and with-reference prices in the details breakdown.
- [ ] Keep group pricing based on the same parsed tier data.

### Task 3: Add translations and validate the frontend

**Files:**
- Modify through script: `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

- [ ] Add all new UI keys through `scripts/add-missing-keys.mjs`.
- [ ] Run `bun run i18n:sync`.
- [ ] Run focused tests, typecheck, lint on touched files, and production build.

### Task 4: Deploy master only

**Files:**
- Remote compose: `/root/gateway/master/docker-compose.yml` (validate only; do not edit)

- [ ] Inspect the remote master service and current image.
- [ ] Build a Linux AMD64 image from the validated local source.
- [ ] Back up the current master image and retain the two newest master backups.
- [ ] Load the new image and force-recreate only `new-api-master`.
- [ ] Verify container health, `/api/status`, and the public pricing page.

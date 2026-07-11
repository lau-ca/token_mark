import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getTierUnitPrice,
  isRequestDependentBillingExpr,
  parseTiersFromExpr,
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
} from './billing-expr'
import { canUseVisualEditor, tryParseVisualConfig } from './tier-expr'

describe('billing expression request-price parsing', () => {
  test('parses per-request and per-second prices through nested calls', () => {
    const requestTier = parseTiersFromExpr('tier("720p", per_request(3.5))')[0]
    const secondTier = parseTiersFromExpr(
      'tier("1080p", per_request(0.9) * param("duration"))'
    )[0]
    const reversedSecondTier = parseTiersFromExpr(
      'tier("4k", param("duration")*per_request(2))'
    )[0]

    assert.deepEqual(getTierUnitPrice(requestTier), {
      unit: 'request',
      price: 3.5,
    })
    assert.deepEqual(getTierUnitPrice(secondTier), {
      unit: 'second',
      price: 0.9,
    })
    assert.deepEqual(getTierUnitPrice(reversedSecondTier), {
      unit: 'second',
      price: 2,
    })
  })

  test('parses the complete Seedance resolution matrix with balanced parentheses', () => {
    const tiers = parseTiersFromExpr(`
      param("resolution") == "480p"
        ? tier("480p", per_request(5.5))
        : param("resolution") == "720p"
          ? tier("720p", per_request(8))
          : param("resolution") == "1080p"
            ? tier("1080p", per_request(0.9) * param("duration"))
            : param("resolution") == "4k"
              ? tier("4k", per_request(2) * param("duration"))
              : tier("invalid", -1)
    `)

    assert.deepEqual(
      tiers.map((tier) => [tier.label, getTierUnitPrice(tier)]),
      [
        ['480p', { unit: 'request', price: 5.5 }],
        ['720p', { unit: 'request', price: 8 }],
        ['1080p', { unit: 'second', price: 0.9 }],
        ['4k', { unit: 'second', price: 2 }],
      ]
    )
  })

  test('retains token and legacy numeric expression parsing', () => {
    const tokenTier = parseTiersFromExpr(
      'tier("base", p * 2.5 + c * 15 + cr * 0.25)'
    )[0]
    const legacyRequestTier = parseTiersFromExpr('tier("legacy", 2500000)')[0]

    assert.equal(tokenTier.inputPrice, 2.5)
    assert.equal(tokenTier.outputPrice, 15)
    assert.equal(tokenTier.cacheReadPrice, 0.25)
    assert.deepEqual(getTierUnitPrice(legacyRequestTier), {
      unit: 'request',
      price: 2.5,
    })
  })

  test('parses historical bare and parenthesized token variables', () => {
    const defaultTier = parseTiersFromExpr('tier("default", p)')[0]
    const legacyTier = parseTiersFromExpr('tier("legacy", (p) * 2 + c * 8)')[0]

    assert.equal(defaultTier.inputPrice, 1)
    assert.equal(legacyTier.inputPrice, 2)
    assert.equal(legacyTier.outputPrice, 8)
  })

  test('retains token tier conditions and request-rule splitting', () => {
    const tiers = parseTiersFromExpr(
      'len <= 200000 ? tier("standard", p * 3 + c * 15) : tier("long", p * 6 + c * 22.5)'
    )
    const split = splitBillingExprAndRequestRules(
      '(tier("base", p * 2 + c * 4))*(param("service_tier") == "priority" ? 2 : 1)'
    )

    assert.deepEqual(tiers[0].conditions, [
      { var: 'len', op: '<=', value: 200000 },
    ])
    assert.deepEqual(tiers[1].conditions, [])
    assert.equal(split.billingExpr, 'tier("base", p * 2 + c * 4)')
    assert.notEqual(tryParseRequestRuleExpr(split.requestRuleExpr), null)
  })

  test('does not mistake numeric-first token multiplication for a request price', () => {
    const tiers = parseTiersFromExpr('tier("base", 2.5 * p)')

    assert.deepEqual(tiers, [])
  })

  test('falls back to raw display for unsupported request-price arithmetic', () => {
    const expressions = [
      'tier("1080p", per_request(0.9) * param("duration") * 2)',
      'tier("base", per_request(2)) + per_request(3)',
      'tier("base", p * 2) * 3',
      'tier("base", p * 2 + p * 3)',
    ]

    for (const expression of expressions) {
      assert.deepEqual(parseTiersFromExpr(expression), [])
    }
  })

  test('ignores request function names inside quoted tier labels', () => {
    const expr = 'tier("param(\\"duration\\")", p * 2 + c * 4)'

    assert.equal(isRequestDependentBillingExpr(expr), false)
    assert.equal(parseTiersFromExpr(expr)[0].label, 'param("duration")')
  })
})

describe('visual editor safety', () => {
  test('keeps request-dependent and unparseable expressions raw-only', () => {
    const requestExpr =
      'param("resolution") == "720p" ? tier("720p", per_request(3.5)) : tier("invalid", -1)'

    assert.equal(isRequestDependentBillingExpr(requestExpr), true)
    assert.equal(tryParseVisualConfig(requestExpr), null)
    assert.equal(canUseVisualEditor(requestExpr), false)
    assert.equal(canUseVisualEditor('tier("base", max(p, c))'), false)
  })

  test('keeps existing visual token expressions editable', () => {
    const expr = 'tier("base", p * 2.5 + c * 15)'

    assert.notEqual(tryParseVisualConfig(expr), null)
    assert.equal(canUseVisualEditor(expr), true)
    assert.equal(canUseVisualEditor(''), true)
  })
})

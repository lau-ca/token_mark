import assert from 'node:assert/strict'
import { Buffer } from 'node:buffer'
import { describe, test } from 'node:test'

import { getTieredBillingSummary } from './format'

function encodeExpr(expr: string): string {
  return Buffer.from(expr, 'utf8').toString('base64')
}

describe('tiered billing log summaries', () => {
  test('reports the matched per-request price', () => {
    const summary = getTieredBillingSummary({
      billing_mode: 'tiered_expr',
      expr_b64: encodeExpr('tier("720p", per_request(3.5))'),
      matched_tier: '720p',
    })

    assert.deepEqual(summary?.unitPrice, { unit: 'request', price: 3.5 })
    assert.deepEqual(summary?.priceEntries, [])
  })

  test('reports the matched per-second price', () => {
    const summary = getTieredBillingSummary({
      billing_mode: 'tiered_expr',
      expr_b64: encodeExpr(
        'tier("1080p", param("duration") * per_request(0.9))'
      ),
      matched_tier: '1080p',
    })

    assert.deepEqual(summary?.unitPrice, { unit: 'second', price: 0.9 })
    assert.deepEqual(summary?.priceEntries, [])
  })

  test('does not invent a structured summary for unsupported request arithmetic', () => {
    const summary = getTieredBillingSummary({
      billing_mode: 'tiered_expr',
      expr_b64: encodeExpr(
        'tier("1080p", per_request(0.9) * param("duration") * 2)'
      ),
      matched_tier: '1080p',
    })

    assert.equal(summary, null)
  })
})

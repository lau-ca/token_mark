import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { parseTiersFromExpr } from './billing-expr'
import {
  getDynamicPriceEntries,
  getDynamicPriceUnitLabel,
} from './dynamic-price'

const OPTIONS = { tokenUnit: 'M' as const }
const translate = (key: string) => key

describe('dynamic pricing display entries', () => {
  test('uses request and second units for request-price tiers', () => {
    const requestTier = parseTiersFromExpr('tier("720p", per_request(3.5))')[0]
    const secondTier = parseTiersFromExpr(
      'tier("1080p", per_request(0.9) * param("duration"))'
    )[0]
    const requestEntry = getDynamicPriceEntries(requestTier, OPTIONS)[0]
    const secondEntry = getDynamicPriceEntries(secondTier, OPTIONS)[0]

    assert.equal(requestEntry.value, 3.5)
    assert.equal(requestEntry.unit, 'request')
    assert.equal(
      getDynamicPriceUnitLabel(requestEntry, '1M', translate),
      'request'
    )
    assert.equal(secondEntry.value, 0.9)
    assert.equal(secondEntry.unit, 'second')
    assert.equal(getDynamicPriceUnitLabel(secondEntry, '1M', translate), 's')
  })

  test('retains token units for existing expressions', () => {
    const tier = parseTiersFromExpr('tier("base", p * 2.5 + c * 15)')[0]
    const entries = getDynamicPriceEntries(tier, OPTIONS)

    assert.deepEqual(
      entries.map((entry) => [entry.field, entry.value, entry.unit]),
      [
        ['inputPrice', 2.5, 'token'],
        ['outputPrice', 15, 'token'],
      ]
    )
    assert.equal(getDynamicPriceUnitLabel(entries[0], '1M', translate), '1M')
  })
})

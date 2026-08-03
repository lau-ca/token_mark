import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { KeyQuotaDataItem } from '../types'
import { aggregateKeyUsage, buildKeyChartData } from './key-usage'

const rows: KeyQuotaDataItem[] = [
  {
    token_id: 11,
    token_name: 'primary',
    masked_key: 'prim**********-key',
    token_status: 1,
    accessed_time: 2000,
    created_at: 1000,
    count: 2,
    token_used: 40,
    quota: 100,
  },
  {
    token_id: 11,
    token_name: 'primary',
    masked_key: 'prim**********-key',
    token_status: 1,
    accessed_time: 2000,
    created_at: 1100,
    count: 1,
    token_used: 20,
    quota: 50,
  },
  {
    token_id: 12,
    token_name: 'backup',
    masked_key: 'back**********-key',
    token_status: 2,
    accessed_time: 0,
    created_at: 0,
    count: 0,
    token_used: 0,
    quota: 0,
  },
  {
    token_id: 13,
    created_at: 1200,
    count: 1,
    token_used: 10,
    quota: 25,
    deleted: true,
  },
]

describe('key usage analytics transforms', () => {
  test('aggregates every key and computes quota share', () => {
    const data = aggregateKeyUsage(rows, (id) => `Deleted Key (${id})`)

    assert.equal(data.length, 3)
    assert.deepEqual(data[0], {
      token_id: 11,
      token_name: 'primary',
      masked_key: 'prim**********-key',
      token_status: 1,
      accessed_time: 2000,
      deleted: false,
      count: 3,
      token_used: 60,
      quota: 150,
      share: 150 / 175,
    })
    assert.equal(data[1].token_name, 'Deleted Key (13)')
    assert.equal(data[1].share, 25 / 175)
    assert.equal(data[2].token_name, 'backup')
    assert.equal(data[2].quota, 0)
  })

  test('maps key identity into model-compatible chart rows', () => {
    const data = buildKeyChartData(rows, (id) => `Deleted Key (${id})`)

    assert.deepEqual(data[0], {
      created_at: 1000,
      model_name: 'primary · prim**********-key',
      count: 2,
      token_used: 40,
      quota: 100,
    })
    assert.equal(data.length, 3)
    assert.equal(data[2].model_name, 'Deleted Key (13)')
  })
})

import { describe, expect, test } from 'bun:test'

import {
  aggregateChannelsByGroup,
  isGroupAggregateRow,
  isTagAggregateRow,
} from './channel-utils.ts'

describe('channel group mode', () => {
  test('groups multi-group channels and deduplicates repeated channel rows', () => {
    const first = {
      id: 1,
      group: 'vip,default',
      used_quota: 10,
      response_time: 100,
      status: 1,
    }
    const second = {
      id: 2,
      group: 'vip',
      used_quota: 20,
      response_time: 300,
      status: 0,
    }

    const rows = aggregateChannelsByGroup([first, first, second])
    const vip = rows.find((row) => row.aggregateValue === 'vip')

    expect(rows.map((row) => row.aggregateValue)).toEqual(['default', 'vip'])
    expect(vip.children.map((channel) => channel.id)).toEqual([1, 2])
    expect(vip.used_quota).toBe(30)
    expect(vip.response_time).toBe(200)
    expect(isGroupAggregateRow(vip)).toBe(true)
    expect(isTagAggregateRow(vip)).toBe(false)
  })
})

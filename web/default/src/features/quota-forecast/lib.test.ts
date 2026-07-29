import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  formatQuotaForecastDuration,
  getQuotaForecastStatusLabel,
  getQuotaForecastTone,
} from './lib'
import type { QuotaForecastResult } from './types'

const t = ((key: string, values?: Record<string, number>) => {
  if (!values) return key
  return Object.entries(values).reduce(
    (result, [name, value]) => result.replace(`{{${name}}}`, String(value)),
    key
  )
}) as never

function predicted(remainingSeconds: number): QuotaForecastResult {
  return {
    user_id: 1,
    status: 'predicted',
    calculated_at: 1,
    predicted_exhausted_at: 1 + remainingSeconds,
    weighted_daily_usage: 100,
    remaining_seconds: remainingSeconds,
    sample_hours: 168,
  }
}

describe('quota forecast presentation', () => {
  test('formats durations without false precision', () => {
    assert.equal(
      formatQuotaForecastDuration(3 * 86400 + 5 * 3600, t),
      'About 3 days 5 hours'
    )
    assert.equal(
      formatQuotaForecastDuration(2 * 3600 + 20 * 60, t),
      'About 2 hours 20 minutes'
    )
    assert.equal(
      formatQuotaForecastDuration(366 * 86400, t),
      'More than one year'
    )
  })

  test('selects threshold tones', () => {
    assert.equal(getQuotaForecastTone(predicted(86400)), 'destructive')
    assert.equal(getQuotaForecastTone(predicted(2 * 86400)), 'warning')
    assert.equal(getQuotaForecastTone(predicted(4 * 86400)), 'muted')
  })

  test('does not predict inactive users', () => {
    assert.equal(
      getQuotaForecastStatusLabel(
        {
          user_id: 1,
          status: 'no_recent_usage',
          calculated_at: 1,
          sample_hours: 168,
        },
        t
      ),
      'No recent usage, no forecast'
    )
  })
})

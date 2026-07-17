import { describe, expect, test } from 'bun:test'

import {
  formatChannelHealthCount,
  formatChannelHealthErrorRate,
  getBeijingDate,
  getBalanceVariant,
  getChannelHealthConfig,
  getCurrentChannelHealthStatus,
} from './channel-utils.ts'

describe('channel health display', () => {
  test('uses the Beijing calendar day around UTC midnight boundaries', () => {
    expect(getBeijingDate(new Date('2026-07-16T15:59:59Z'))).toBe('2026-07-16')
    expect(getBeijingDate(new Date('2026-07-16T16:00:00Z'))).toBe('2026-07-17')
  })

  test('marks a previous Beijing-day snapshot as pending', () => {
    const now = new Date('2026-07-17T03:00:00Z')
    expect(getCurrentChannelHealthStatus('2026-07-16', 'healthy', now)).toBe(
      'pending'
    )
    expect(getCurrentChannelHealthStatus('2026-07-17', 'healthy', now)).toBe(
      'healthy'
    )
  })

  test('maps persisted states to their display labels and colors', () => {
    expect(getChannelHealthConfig('healthy')).toEqual({
      labelKey: 'Healthy',
      variant: 'success',
    })
    expect(getChannelHealthConfig('warning').variant).toBe('warning')
    expect(getChannelHealthConfig('critical').variant).toBe('danger')
    expect(getChannelHealthConfig('unknown').labelKey).toBe(
      'Insufficient samples'
    )
    expect(getChannelHealthConfig('pending').labelKey).toBe('Pending refresh')
  })

  test('formats rates and call counts with tabular display values', () => {
    expect(formatChannelHealthErrorRate(2, 'en-US')).toBe('2.0%')
    expect(formatChannelHealthErrorRate(2.345, 'en-US')).toBe('2.35%')
    expect(formatChannelHealthCount(12345, 'en-US')).toBe('12,345')
  })

  test('shows configured balance warning thresholds after the first refresh', () => {
    expect(getBalanceVariant(0, 0)).toBe('neutral')
    expect(getBalanceVariant(19.99, 1)).toBe('danger')
    expect(getBalanceVariant(20, 1)).toBe('warning')
    expect(getBalanceVariant(49.99, 1)).toBe('warning')
    expect(getBalanceVariant(50, 1)).toBe('success')
  })
})

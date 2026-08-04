import type { TFunction } from 'i18next'

import type { QuotaForecastResult } from './types'

export type QuotaForecastTone = 'destructive' | 'warning' | 'muted'

const DAY_SECONDS = 24 * 60 * 60
const YEAR_SECONDS = 365 * DAY_SECONDS

export function getQuotaForecastTone(
  forecast?: QuotaForecastResult
): QuotaForecastTone {
  if (forecast?.status === 'depleted') return 'destructive'
  if (forecast?.status !== 'predicted' || !forecast.remaining_seconds) {
    return 'muted'
  }
  if (forecast.remaining_seconds <= DAY_SECONDS) return 'destructive'
  if (forecast.remaining_seconds <= 3 * DAY_SECONDS) return 'warning'
  return 'muted'
}

export function formatQuotaForecastDuration(
  seconds: number | undefined,
  t: TFunction
): string {
  if (!seconds || seconds <= 0) return t('Forecast unavailable')
  if (seconds > YEAR_SECONDS) return t('More than one year')

  const days = Math.floor(seconds / DAY_SECONDS)
  const hours = Math.floor((seconds % DAY_SECONDS) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  if (days > 0) {
    return t('About {{days}} days {{hours}} hours', { days, hours })
  }
  if (hours > 0) {
    return t('About {{hours}} hours {{minutes}} minutes', { hours, minutes })
  }
  return t('About {{minutes}} minutes', { minutes: Math.max(1, minutes) })
}

export function getQuotaForecastStatusLabel(
  forecast: QuotaForecastResult | undefined,
  t: TFunction
): string {
  if (!forecast) return t('Forecast unavailable')
  switch (forecast.status) {
    case 'depleted':
      return t('Balance depleted')
    case 'sampling':
      return t('Collecting usage data')
    case 'no_recent_usage':
      return t('No recent usage, no forecast')
    case 'predicted':
      return formatQuotaForecastDuration(forecast.remaining_seconds, t)
    default:
      return t('Forecast unavailable')
  }
}

import { api } from '@/lib/api'

import type { QuotaForecastApiResponse, QuotaForecastResult } from './types'

export const SELF_QUOTA_FORECAST_QUERY_KEY = ['quota-forecast', 'self'] as const
export const QUOTA_FORECAST_STALE_TIME = 5 * 60 * 1000

export async function getAdminQuotaForecasts(userIds: number[]) {
  const response = await api.post<
    QuotaForecastApiResponse<QuotaForecastResult[]>
  >('/api/user/quota-forecast', { user_ids: userIds })
  return response.data
}

export async function getSelfQuotaForecast() {
  const response = await api.get<QuotaForecastApiResponse<QuotaForecastResult>>(
    '/api/user/self/quota-forecast'
  )
  return response.data
}

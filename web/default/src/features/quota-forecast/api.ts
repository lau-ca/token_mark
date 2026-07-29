import { api } from '@/lib/api'

import type { QuotaForecastApiResponse, QuotaForecastResult } from './types'

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

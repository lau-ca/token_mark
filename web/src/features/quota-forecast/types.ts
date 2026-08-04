export type QuotaForecastStatus =
  | 'predicted'
  | 'depleted'
  | 'sampling'
  | 'no_recent_usage'
  | 'unavailable'

export interface QuotaForecastResult {
  user_id: number
  status: QuotaForecastStatus
  calculated_at: number
  predicted_exhausted_at?: number
  weighted_daily_usage?: number
  remaining_seconds?: number
  sample_hours: number
}

export interface QuotaForecastApiResponse<T> {
  success: boolean
  message?: string
  data?: T
}

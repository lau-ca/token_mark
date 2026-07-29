/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import {
  getSelfQuotaForecast,
  QUOTA_FORECAST_STALE_TIME,
  SELF_QUOTA_FORECAST_QUERY_KEY,
} from '@/features/quota-forecast/api'
import { getQuotaForecastTone } from '@/features/quota-forecast/lib'
import { QuotaForecastDisplay } from '@/features/quota-forecast/quota-forecast-display'
import { formatLogQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import { useLogsViewScope, useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function StatBadge(props: { label: string; value: ReactNode; accent: string }) {
  return (
    <div className='border-border/60 bg-muted/25 inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <div className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

export function CommonLogsStats() {
  const { t } = useTranslation()
  const { isAdminView: isAdmin } = useLogsViewScope()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  const { data: stats, isLoading } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    queryFn: async () => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })

      const result = isAdmin
        ? await getLogStats(params)
        : await getUserLogStats(params)

      return result.success
        ? result.data || DEFAULT_LOG_STATS
        : DEFAULT_LOG_STATS
    },
    placeholderData: (previousData) => previousData,
  })
  const forecastQuery = useQuery({
    queryKey: SELF_QUOTA_FORECAST_QUERY_KEY,
    queryFn: getSelfQuotaForecast,
    staleTime: QUOTA_FORECAST_STALE_TIME,
    retry: 1,
    enabled: !isAdmin,
  })
  const forecast = forecastQuery.data?.success
    ? forecastQuery.data.data
    : undefined
  const forecastTone = getQuotaForecastTone(forecast)
  let forecastAccent = 'bg-slate-400/70'
  if (forecastTone === 'destructive') {
    forecastAccent = 'bg-rose-500/70'
  } else if (forecastTone === 'warning') {
    forecastAccent = 'bg-amber-500/75'
  }

  if (isLoading) {
    return (
      <div className='flex items-center gap-2'>
        <Skeleton className='h-7 w-[150px] rounded-md' />
        <Skeleton className='h-7 w-[100px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
      </div>
    )
  }

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <StatBadge
        label={t('Usage')}
        value={sensitiveVisible ? formatLogQuota(stats?.quota || 0) : '••••'}
        accent='bg-sky-500/70'
      />
      {!isAdmin ? (
        <StatBadge
          label={t('Runway')}
          value={
            <QuotaForecastDisplay
              forecast={forecast}
              isLoading={forecastQuery.isLoading}
              isError={
                forecastQuery.isError || forecastQuery.data?.success === false
              }
              variant='stats'
            />
          }
          accent={forecastAccent}
        />
      ) : null}
      <StatBadge
        label={t('RPM')}
        value={stats?.rpm || 0}
        accent='bg-rose-500/65'
      />
      <StatBadge
        label={t('TPM')}
        value={stats?.tpm || 0}
        accent='bg-slate-400/70'
      />
    </div>
  )
}

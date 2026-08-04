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
import { CircleAlert } from 'lucide-react'
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { getUserKeyQuotaDates } from '@/features/dashboard/api'
import {
  aggregateKeyUsage,
  buildKeyChartData,
  buildQueryParams,
  getDefaultDays,
} from '@/features/dashboard/lib'
import type {
  DashboardChartPreferences,
  DashboardFilters,
} from '@/features/dashboard/types'
import { computeTimeRange } from '@/lib/time'

import { ConsumptionDistributionChart } from '../models/consumption-distribution-chart'
import { LogStatCards } from '../models/log-stat-cards'
import { ModelCharts } from '../models/model-charts'
import { KeyUsageTable } from './key-usage-table'

interface KeyUsageAnalyticsProps {
  filters: DashboardFilters
  preferences: DashboardChartPreferences
}

export function KeyUsageAnalytics({
  filters,
  preferences,
}: KeyUsageAnalyticsProps) {
  const { t } = useTranslation()
  const queryParams = useMemo(() => {
    const timeRange = computeTimeRange(
      getDefaultDays(filters.time_granularity),
      filters.start_timestamp,
      filters.end_timestamp
    )
    return buildQueryParams(timeRange, filters)
  }, [filters])

  const query = useQuery({
    queryKey: ['dashboard', 'key-usage', queryParams],
    queryFn: async () => {
      const response = await getUserKeyQuotaDates(queryParams)
      if (!response.success) {
        throw new Error(response.message || t('Failed to load'))
      }
      return response.data || []
    },
  })

  const deletedKeyLabel = useCallback(
    (tokenId: number) => t('Deleted Key ({{id}})', { id: tokenId }),
    [t]
  )
  const rows = useMemo(() => query.data || [], [query.data])
  const chartData = useMemo(
    () => buildKeyChartData(rows, deletedKeyLabel),
    [deletedKeyLabel, rows]
  )
  const tableData = useMemo(
    () => aggregateKeyUsage(rows, deletedKeyLabel),
    [deletedKeyLabel, rows]
  )

  return (
    <div className='space-y-3 sm:space-y-4'>
      <LogStatCards filters={filters} />
      {query.isError ? (
        <Alert variant='destructive'>
          <CircleAlert />
          <AlertTitle>{t('Failed to load Key usage')}</AlertTitle>
          <AlertDescription>
            {query.error instanceof Error
              ? query.error.message
              : t('Please try again later.')}
          </AlertDescription>
        </Alert>
      ) : (
        <>
          <ConsumptionDistributionChart
            data={chartData}
            loading={query.isLoading}
            defaultChartType={preferences.consumptionDistributionChart}
            timeGranularity={filters.time_granularity}
          />
          <ModelCharts
            data={chartData}
            loading={query.isLoading}
            defaultChartTab={preferences.modelAnalyticsChart}
            timeGranularity={filters.time_granularity}
            titleKey='Key Usage Analytics'
          />
          <KeyUsageTable data={tableData} loading={query.isLoading} />
        </>
      )}
    </div>
  )
}

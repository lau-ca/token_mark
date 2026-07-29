import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { cn } from '@/lib/utils'

import { getQuotaForecastStatusLabel, getQuotaForecastTone } from './lib'
import type { QuotaForecastResult } from './types'

interface QuotaForecastDisplayProps {
  forecast?: QuotaForecastResult
  isLoading?: boolean
  isError?: boolean
  variant?: 'table' | 'dashboard' | 'stats'
  className?: string
}

const toneClasses = {
  destructive: 'text-destructive',
  warning: 'text-warning',
  muted: 'text-muted-foreground',
} as const

export function QuotaForecastDisplay(props: QuotaForecastDisplayProps) {
  const { t } = useTranslation()
  if (props.isLoading) {
    return (
      <Skeleton
        className={cn(
          'h-3.5 rounded-sm',
          props.variant === 'stats' ? 'w-32' : 'w-28'
        )}
      />
    )
  }

  const forecast = props.isError ? undefined : props.forecast
  const label = getQuotaForecastStatusLabel(forecast, t)
  const tone = getQuotaForecastTone(forecast)
  const predictedExhaustedAt =
    forecast?.status === 'predicted'
      ? forecast.predicted_exhausted_at
      : undefined

  const details = predictedExhaustedAt ? (
    <div className='space-y-1 text-xs'>
      <div>
        {t('Expected exhaustion:')} {formatTimestamp(predictedExhaustedAt)}
      </div>
      <div>
        {t('Weighted daily usage:')}{' '}
        {formatQuota(forecast?.weighted_daily_usage || 0)}
      </div>
      <div>{t('Based on the weighted average of the last 7 days')}</div>
      <div>{t('Assumes no recharge and a stable usage trend')}</div>
    </div>
  ) : null

  if (props.variant === 'stats') {
    const statsContent = (
      <span
        className={cn(
          'inline-flex whitespace-nowrap text-xs leading-4 font-semibold tabular-nums',
          toneClasses[tone],
          props.className
        )}
      >
        {predictedExhaustedAt ? formatTimestamp(predictedExhaustedAt) : label}
      </span>
    )

    if (!details) return statsContent

    return (
      <Tooltip>
        <TooltipTrigger render={<span className='cursor-help' />}>
          {statsContent}
        </TooltipTrigger>
        <TooltipContent>{details}</TooltipContent>
      </Tooltip>
    )
  }

  const content = (
    <div
      className={cn(
        'min-w-0',
        props.variant === 'dashboard'
          ? 'space-y-0.5'
          : 'max-w-[150px] truncate text-[11px] leading-4',
        toneClasses[tone],
        props.className
      )}
    >
      <div className='font-medium tabular-nums'>{label}</div>
      {props.variant === 'dashboard' && predictedExhaustedAt && (
        <div className='text-muted-foreground text-[10px] leading-4 font-normal tracking-tight whitespace-nowrap tabular-nums sm:text-[11px]'>
          {t('Expected {{time}}', {
            time: formatTimestamp(predictedExhaustedAt),
          })}
        </div>
      )}
      {props.variant !== 'dashboard' && predictedExhaustedAt && (
        <div className='truncate'>
          {t('Expected {{time}}', {
            time: formatTimestamp(predictedExhaustedAt),
          })}
        </div>
      )}
    </div>
  )

  if (!details) {
    return content
  }

  return (
    <Tooltip>
      <TooltipTrigger render={<div className='cursor-help' />}>
        {content}
      </TooltipTrigger>
      <TooltipContent>{details}</TooltipContent>
    </Tooltip>
  )
}

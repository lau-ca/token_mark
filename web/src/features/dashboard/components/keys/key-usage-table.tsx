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
import { ArrowUpDown, KeyRound, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { API_KEY_STATUSES } from '@/features/keys/constants'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'

import type { AggregatedKeyUsage } from '../../types'

type SortKey = 'quota' | 'count' | 'token_used' | 'share' | 'accessed_time'

interface KeyUsageTableProps {
  data: AggregatedKeyUsage[]
  loading?: boolean
}

export function KeyUsageTable({ data, loading }: KeyUsageTableProps) {
  const { t, i18n } = useTranslation()
  const [search, setSearch] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('quota')
  const [sortDescending, setSortDescending] = useState(true)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  const filteredData = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    return data
      .filter((item) => {
        if (!keyword) return true
        return (
          item.token_name.toLowerCase().includes(keyword) ||
          item.masked_key.toLowerCase().includes(keyword) ||
          String(item.token_id).includes(keyword)
        )
      })
      .sort((a, b) => {
        const difference = a[sortKey] - b[sortKey]
        return sortDescending ? -difference : difference
      })
  }, [data, search, sortDescending, sortKey])

  const handleSort = (nextSortKey: SortKey) => {
    if (nextSortKey === sortKey) {
      setSortDescending((value) => !value)
      return
    }
    setSortKey(nextSortKey)
    setSortDescending(true)
  }

  const sortableHead = (label: string, key: SortKey) => (
    <Button
      variant='ghost'
      size='sm'
      className='-ml-2 h-8 px-2'
      onClick={() => handleSort(key)}
    >
      {label}
      <ArrowUpDown className='ml-1 size-3.5' />
    </Button>
  )

  let tableContent = (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className='pl-4 sm:pl-5'>{t('API Key')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead className='text-right'>
            {sortableHead(t('Requests'), 'count')}
          </TableHead>
          <TableHead className='text-right'>
            {sortableHead(t('Tokens'), 'token_used')}
          </TableHead>
          <TableHead className='text-right'>
            {sortableHead(t('Consumption Amount'), 'quota')}
          </TableHead>
          <TableHead className='text-right'>
            {sortableHead(t('Share'), 'share')}
          </TableHead>
          <TableHead>{sortableHead(t('Last Used'), 'accessed_time')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {filteredData.map((item) => {
          const status = API_KEY_STATUSES[item.token_status]
          let statusContent = <span>-</span>
          if (item.deleted) {
            statusContent = (
              <StatusBadge
                label={t('Deleted')}
                variant='danger'
                copyable={false}
              />
            )
          } else if (status) {
            statusContent = (
              <StatusBadge
                label={t(status.label)}
                variant={status.variant}
                copyable={false}
              />
            )
          }

          return (
            <TableRow key={item.token_id}>
              <TableCell className='min-w-56 pl-4 sm:pl-5'>
                <div className='font-medium'>{item.token_name}</div>
                <div className='text-muted-foreground font-mono text-xs'>
                  {item.masked_key || `ID ${item.token_id}`}
                </div>
              </TableCell>
              <TableCell>{statusContent}</TableCell>
              <TableCell className='text-right'>
                {formatNumber(item.count, locale)}
              </TableCell>
              <TableCell className='text-right'>
                {formatNumber(item.token_used, locale)}
              </TableCell>
              <TableCell className='text-right font-medium'>
                {formatQuota(item.quota)}
              </TableCell>
              <TableCell className='text-right'>
                {Intl.NumberFormat(locale, {
                  style: 'percent',
                  maximumFractionDigits: 1,
                }).format(item.share)}
              </TableCell>
              <TableCell className='text-muted-foreground'>
                {formatTimestamp(item.accessed_time)}
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
  if (loading) {
    tableContent = (
      <div className='space-y-2 p-4'>
        {Array.from({ length: 5 }, (_, index) => (
          <Skeleton key={index} className='h-12 w-full' />
        ))}
      </div>
    )
  } else if (filteredData.length === 0) {
    tableContent = (
      <Empty className='min-h-64 border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <KeyRound />
          </EmptyMedia>
          <EmptyTitle>
            {search ? t('No matching API keys') : t('No API keys')}
          </EmptyTitle>
          <EmptyDescription>
            {search
              ? t('Try another key name or masked key.')
              : t(
                  'No API keys available. Create your first API key to get started.'
                )}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-col gap-2 border-b px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:px-5 sm:py-3'>
        <div className='flex items-center gap-2'>
          <IconBadge tone='chart-2' size='sm'>
            <KeyRound />
          </IconBadge>
          <div className='text-sm font-semibold'>{t('Key Usage Details')}</div>
          <span className='text-muted-foreground text-xs'>
            {t('{{count}} keys', { count: data.length })}
          </span>
        </div>
        <div className='relative w-full sm:w-64'>
          <Search className='text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('Search key name or masked key')}
            className='h-8 pl-8'
          />
        </div>
      </div>

      {tableContent}
    </div>
  )
}

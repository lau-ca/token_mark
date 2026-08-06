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
import dayjs from '@/lib/dayjs'

import type {
  KeyUsageExportData,
  KeyUsageExportKey,
  KeyUsageExportModel,
} from '../types'

export interface KeyUsageExportKeyRow extends KeyUsageExportKey {
  share: number
}

export interface KeyUsageExportModelRow extends KeyUsageExportModel {
  key_share: number
}

export interface KeyUsageExportTotals {
  active_keys: number
  model_count: number
  request_count: number
  total_tokens: number
  quota: number
}

export interface KeyUsageExportReport {
  generated_at: number
  start_timestamp: number
  end_timestamp: number
  keys: KeyUsageExportKeyRow[]
  models: KeyUsageExportModelRow[]
  totals: KeyUsageExportTotals
}

interface KeyUsageExportLabels {
  deletedKey: (tokenId: number) => string
  unnamedKey: (tokenId: number) => string
  unknownModel: string
}

export function buildKeyUsageExportReport(
  data: KeyUsageExportData,
  labels: KeyUsageExportLabels
): KeyUsageExportReport {
  const totalQuota = data.keys.reduce(
    (sum, item) => sum + (Number(item.quota) || 0),
    0
  )
  const keyQuotaById = new Map<number, number>()
  const keys = data.keys
    .map((item) => {
      const quota = Number(item.quota) || 0
      keyQuotaById.set(item.token_id, quota)
      let tokenName = item.token_name
      if (item.deleted) {
        tokenName = labels.deletedKey(item.token_id)
      } else if (!tokenName) {
        tokenName = labels.unnamedKey(item.token_id)
      }
      return {
        ...item,
        token_name: tokenName,
        request_count: Number(item.request_count) || 0,
        total_tokens: Number(item.total_tokens) || 0,
        quota,
        model_count: Number(item.model_count) || 0,
        last_used_at: Number(item.last_used_at) || 0,
        share: totalQuota > 0 ? quota / totalQuota : 0,
      }
    })
    .sort((a, b) => b.quota - a.quota || a.token_id - b.token_id)

  const keyOrder = new Map(keys.map((item, index) => [item.token_id, index]))
  const models = data.models
    .map((item) => {
      const keyQuota = keyQuotaById.get(item.token_id) || 0
      let tokenName = item.token_name
      if (item.deleted) {
        tokenName = labels.deletedKey(item.token_id)
      } else if (!tokenName) {
        tokenName = labels.unnamedKey(item.token_id)
      }
      return {
        ...item,
        token_name: tokenName,
        model_name: item.model_name || labels.unknownModel,
        request_count: Number(item.request_count) || 0,
        total_tokens: Number(item.total_tokens) || 0,
        quota: Number(item.quota) || 0,
        last_used_at: Number(item.last_used_at) || 0,
        key_share: keyQuota > 0 ? (Number(item.quota) || 0) / keyQuota : 0,
      }
    })
    .sort((a, b) => {
      const orderDifference =
        (keyOrder.get(a.token_id) ?? Number.MAX_SAFE_INTEGER) -
        (keyOrder.get(b.token_id) ?? Number.MAX_SAFE_INTEGER)
      if (orderDifference !== 0) return orderDifference
      return b.quota - a.quota || a.model_name.localeCompare(b.model_name)
    })

  return {
    generated_at: data.generated_at,
    start_timestamp: data.start_timestamp,
    end_timestamp: data.end_timestamp,
    keys,
    models,
    totals: {
      active_keys: keys.filter((item) => item.request_count > 0).length,
      model_count: new Set(models.map((item) => item.model_name)).size,
      request_count: keys.reduce((sum, item) => sum + item.request_count, 0),
      total_tokens: keys.reduce((sum, item) => sum + item.total_tokens, 0),
      quota: totalQuota,
    },
  }
}

export function sanitizeExportFileSegment(value: string): string {
  return (
    value
      .trim()
      .replaceAll(/[\\/:*?"<>|]+/g, '_')
      .replaceAll(/\s+/g, '-') || 'user'
  )
}

export function buildKeyUsageExportFileName(
  reportTitle: string,
  username: string,
  startTimestamp: number,
  endTimestamp: number
): string {
  const startDate = dayjs.unix(startTimestamp).format('YYYY-MM-DD')
  const endDate = dayjs.unix(endTimestamp).format('YYYY-MM-DD')
  return `${sanitizeExportFileSegment(reportTitle)}_${sanitizeExportFileSegment(username)}_${startDate}_${endDate}.xlsx`
}

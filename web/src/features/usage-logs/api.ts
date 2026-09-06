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
import { api } from '@/lib/api'

import { mergeLogFilterModels } from './lib/filter-options'
import { parseTaskArtifactsResponse } from './lib/task-artifacts'
import type { TaskArtifactsResponse } from './types'

const taskArtifactRequestConfig = { skipBusinessError: true, skipErrorHandler: true } as const

export async function getTaskArtifacts(taskId: string) {
  const response = await api.get<TaskArtifactsResponse>(
    `/api/task/${encodeURIComponent(taskId)}/artifacts`,
    taskArtifactRequestConfig
  )
  return parseTaskArtifactsResponse(response.data)
}
import type {
  CommonLogFilterOptions,
  GetLogsParams,
  GetLogsResponse,
  GetLogStatsParams,
  GetLogStatsResponse,
  GetMidjourneyLogsParams,
  GetTaskLogsParams,
  UserInfo,
} from './types'

type PricingFilterItem = {
  model_name: string
  enable_groups?: string[]
}

type CompositeFilterItem = {
  name: string
  public_model: string
}

type UserGroupFilterItem = {
  composite?: boolean
  public_model?: string
}

type ChannelFilterItem = {
  id: number
  name: string
}

function buildQueryParams(params: Record<string, unknown>): URLSearchParams {
  const queryParams = new URLSearchParams()

  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      queryParams.append(key, String(value))
    }
  }

  return queryParams
}

// ============================================================================
// Generic API Helpers
// ============================================================================

function buildApiPath(endpoint: string, isAdmin: boolean): string {
  return isAdmin ? endpoint : `${endpoint}/self`
}

async function fetchLogs<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogsResponse> {
  const paramRecord = params as unknown as Record<string, unknown>
  const queryParams = buildQueryParams({
    p: paramRecord.p || 1,
    page_size: paramRecord.page_size || 20,
    ...params,
  })
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}?${queryParams}`)
  return res.data
}

async function fetchLogStats<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogStatsResponse> {
  const queryParams = buildQueryParams(
    params as unknown as Record<string, unknown>
  )
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}/stat?${queryParams}`)
  return res.data
}

// ============================================================================
// Common Log APIs
// ============================================================================

export const getAllLogs = (params: GetLogsParams = {}) =>
  fetchLogs('/api/log', params, true)

export const getUserLogs = (
  params: Omit<GetLogsParams, 'username' | 'channel'> = {}
) => fetchLogs('/api/log', params, false)

export const getLogStats = (params: GetLogStatsParams = {}) =>
  fetchLogStats('/api/log', params, true)

export const getUserLogStats = (
  params: Omit<GetLogStatsParams, 'username' | 'channel'> = {}
) => fetchLogStats('/api/log', params, false)

export async function getUserInfo(
  userId: number
): Promise<{ success: boolean; message?: string; data?: UserInfo }> {
  const res = await api.get(`/api/user/${userId}`)
  return res.data
}

async function getAllChannelFilterOptions(): Promise<ChannelFilterItem[]> {
  const firstResponse = await api.get('/api/channel', {
    params: { p: 1, page_size: 100, sort_by: 'id', sort_order: 'asc' },
  })
  const firstPage = firstResponse.data?.data
  const firstItems = (firstPage?.items ?? []) as ChannelFilterItem[]
  const total = Number(firstPage?.total ?? firstItems.length)
  const pageCount = Math.ceil(total / 100)

  if (pageCount <= 1) return firstItems

  const remainingResponses = await Promise.all(
    Array.from({ length: pageCount - 1 }, (_, index) =>
      api.get('/api/channel', {
        params: {
          p: index + 2,
          page_size: 100,
          sort_by: 'id',
          sort_order: 'asc',
        },
      })
    )
  )

  return [
    ...firstItems,
    ...remainingResponses.flatMap(
      (response) => (response.data?.data?.items ?? []) as ChannelFilterItem[]
    ),
  ]
}

export async function getCommonLogFilterOptions(
  isAdmin: boolean
): Promise<CommonLogFilterOptions> {
  const pricingPromise = api.get('/api/pricing')

  if (isAdmin) {
    const [pricingResponse, groupsResponse, compositeResponse, channels] =
      await Promise.all([
        pricingPromise,
        api.get('/api/group/'),
        api.get('/api/composite-groups/'),
        getAllChannelFilterOptions(),
      ])
    const physicalGroups = (groupsResponse.data?.data ?? []) as string[]
    const compositeGroups = (compositeResponse.data?.data ?? []) as
      | CompositeFilterItem[]
      | undefined
    const groups = [
      ...physicalGroups.map((name) => ({ name })),
      ...(compositeGroups ?? []).map((group) => ({
        name: group.name,
        composite: true,
        publicModel: group.public_model,
      })),
    ].sort((a, b) => a.name.localeCompare(b.name))
    const pricingItems = (pricingResponse.data?.data ??
      []) as PricingFilterItem[]
    const models = mergeLogFilterModels(
      pricingItems.map((item) => ({
        name: item.model_name,
        groups: item.enable_groups ?? [],
      })),
      groups
    )

    return { groups, models, channels }
  }

  const [pricingResponse, groupsResponse] = await Promise.all([
    pricingPromise,
    api.get('/api/user/self/groups'),
  ])
  const userGroups = (groupsResponse.data?.data ?? {}) as Record<
    string,
    UserGroupFilterItem
  >
  const groups = Object.entries(userGroups)
    .map(([name, group]) => ({
      name,
      composite: group.composite === true,
      publicModel: group.public_model,
    }))
    .sort((a, b) => a.name.localeCompare(b.name))
  const pricingItems = (pricingResponse.data?.data ?? []) as PricingFilterItem[]
  const models = mergeLogFilterModels(
    pricingItems.map((item) => ({
      name: item.model_name,
      groups: item.enable_groups ?? [],
    })),
    groups
  )

  return { groups, models, channels: [] }
}

// ============================================================================
// MjProxy (Drawing) Logs API
// ============================================================================

export const getAllMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, true)

export const getUserMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, false)

// ============================================================================
// Task Logs API
// ============================================================================

export const getAllTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, true)

export const getUserTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, false)

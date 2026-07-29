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

import type {
  CompositeGroup,
  CompositeGroupApiResponse,
  CompositeGroupOptions,
  CompositeGroupPayload,
} from './types'

type PricingItem = {
  model_name: string
  quota_type: number
  billing_mode?: string
  enable_groups?: string[]
}

function requireSuccess<T>(response: CompositeGroupApiResponse<T>): T {
  if (!response.success) {
    throw new Error(response.message || 'Request failed')
  }
  return response.data
}

function getBillingMode(item: PricingItem) {
  if (item.billing_mode === 'tiered_expr') return 'tiered' as const
  if (item.quota_type === 1) return 'per-request' as const
  return 'per-token' as const
}

export async function listCompositeGroups(): Promise<CompositeGroup[]> {
  const response = await api.get<CompositeGroupApiResponse<CompositeGroup[]>>(
    '/api/composite-groups/'
  )
  return requireSuccess(response.data)
}

export async function createCompositeGroup(
  payload: CompositeGroupPayload
): Promise<CompositeGroup> {
  const response = await api.post<CompositeGroupApiResponse<CompositeGroup>>(
    '/api/composite-groups/',
    payload
  )
  return requireSuccess(response.data)
}

export async function updateCompositeGroup(
  id: number,
  payload: CompositeGroupPayload
): Promise<CompositeGroup> {
  const response = await api.put<CompositeGroupApiResponse<CompositeGroup>>(
    `/api/composite-groups/${id}`,
    payload
  )
  return requireSuccess(response.data)
}

export async function updateCompositeGroupStatus(
  id: number,
  status: number
): Promise<void> {
  const response = await api.patch<CompositeGroupApiResponse<null>>(
    `/api/composite-groups/${id}/status`,
    { status }
  )
  requireSuccess(response.data)
}

export async function validateCompositeGroup(id: number): Promise<void> {
  const response = await api.post<CompositeGroupApiResponse<null>>(
    `/api/composite-groups/${id}/validate`
  )
  requireSuccess(response.data)
}

export async function deleteCompositeGroup(id: number): Promise<void> {
  const response = await api.delete<CompositeGroupApiResponse<null>>(
    `/api/composite-groups/${id}`
  )
  requireSuccess(response.data)
}

export async function getCompositeGroupOptions(): Promise<CompositeGroupOptions> {
  const [groupsResponse, pricingResponse] = await Promise.all([
    api.get<CompositeGroupApiResponse<string[]>>('/api/group/'),
    api.get<{ success: boolean; data: PricingItem[] }>('/api/pricing'),
  ])
  const groups = requireSuccess(groupsResponse.data).sort((a, b) =>
    a.localeCompare(b)
  )
  const pricing = pricingResponse.data.data ?? []
  const models = [
    ...new Map(
      pricing.map((item) => [
        item.model_name,
        {
          name: item.model_name,
          billingMode: getBillingMode(item),
          groups: item.enable_groups ?? [],
        },
      ])
    ).values(),
  ].sort((a, b) => a.name.localeCompare(b.name))
  return { groups, models }
}

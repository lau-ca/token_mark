import { api } from '@/lib/api'

import type {
  AgentConfigPayload,
  AgentProfileData,
  AgentProfileView,
  AgentSettlement,
  AgentStatsData,
  AgentStatsParams,
} from './types'

interface ApiResponse<T> {
  success: boolean
  message?: string
  data?: T
}

export async function getAgentProfiles(
  includeDisabled = false,
  keyword = ''
): Promise<ApiResponse<AgentProfileView[]>> {
  const response = await api.get('/api/agent/admin/profiles', {
    params: { include_disabled: includeDisabled, keyword },
  })
  return response.data
}

export async function getAgentProfile(
  userId?: number
): Promise<ApiResponse<AgentProfileData>> {
  const url = userId
    ? `/api/agent/admin/${userId}/profile`
    : '/api/agent/self/profile'
  const response = await api.get(url)
  return response.data
}

export async function updateAgentProfile(
  userId: number,
  payload: AgentConfigPayload
): Promise<ApiResponse<AgentProfileData>> {
  const response = await api.put(`/api/agent/admin/${userId}/profile`, payload)
  return response.data
}

export async function getAgentStats(
  params: AgentStatsParams,
  userId?: number
): Promise<ApiResponse<AgentStatsData>> {
  const url = userId
    ? `/api/agent/admin/${userId}/stats`
    : '/api/agent/self/stats'
  const response = await api.get(url, { params })
  return response.data
}

export async function getAgentSettlements(
  userId?: number
): Promise<ApiResponse<AgentSettlement[]>> {
  const url = userId
    ? `/api/agent/admin/${userId}/settlements`
    : '/api/agent/self/settlements'
  const response = await api.get(url)
  return response.data
}

export async function previewAgentSettlement(userId: number, cutoff: number) {
  const response = await api.get(
    `/api/agent/admin/${userId}/settlement/preview`,
    { params: { cutoff } }
  )
  return response.data
}

export async function confirmAgentSettlement(
  userId: number,
  cutoff: number,
  paymentReference: string
): Promise<ApiResponse<AgentSettlement>> {
  const response = await api.post(`/api/agent/admin/${userId}/settlement`, {
    cutoff,
    payment_reference: paymentReference,
  })
  return response.data
}

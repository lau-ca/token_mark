import { z } from 'zod'

export const agentGroupMarginSchema = z.object({
  id: z.number().optional(),
  version_id: z.number().optional(),
  group: z.string(),
  gross_margin_rate: z.number(),
  platform_retention_rate: z.number(),
})

export const agentProfileSchema = z.object({
  user_id: z.number(),
  enabled: z.boolean(),
  platform_retention_rate: z.number().optional(),
  remark: z.string().optional(),
  created_at: z.number().optional(),
  updated_at: z.number().optional(),
})

export const agentMarginVersionSchema = z.object({
  id: z.number(),
  agent_user_id: z.number(),
  platform_retention_rate: z.number().optional(),
  effective_from: z.number(),
  group_margins: z.array(agentGroupMarginSchema).optional().default([]),
})

export const agentProfileViewSchema = agentProfileSchema.extend({
  username: z.string(),
  display_name: z.string().optional(),
  role: z.number(),
})

export const agentStatsSummarySchema = z.object({
  customer_count: z.number(),
  consumption_quota: z.number(),
  gross_profit_quota: z.number(),
  platform_retained_quota: z.number().optional(),
  agent_earnings_quota: z.number(),
  settled_quota: z.number(),
  pending_quota: z.number(),
})

export const agentStatsDetailSchema = z.object({
  customer_user_id: z.number(),
  username: z.string(),
  group: z.string(),
  consumption_quota: z.number(),
  gross_margin_rate: z.number().nullable().optional(),
  gross_profit_quota: z.number(),
  platform_retained_quota: z.number().optional(),
  agent_earnings_quota: z.number(),
  configured: z.boolean(),
  unconfigured_quota: z.number().optional(),
})

export const agentSettlementSchema = z.object({
  id: z.number(),
  agent_user_id: z.number(),
  period_start: z.number(),
  period_end: z.number(),
  consumption_quota: z.number(),
  gross_profit_quota: z.number(),
  platform_retained_quota: z.number().optional(),
  agent_earnings_quota: z.number(),
  payment_reference: z.string().optional(),
  confirmed_by: z.number(),
  confirmed_at: z.number(),
})

export type AgentGroupMargin = z.infer<typeof agentGroupMarginSchema>
export type AgentProfile = z.infer<typeof agentProfileSchema>
export type AgentMarginVersion = z.infer<typeof agentMarginVersionSchema>
export type AgentProfileView = z.infer<typeof agentProfileViewSchema>
export type AgentStatsSummary = z.infer<typeof agentStatsSummarySchema>
export type AgentStatsDetail = z.infer<typeof agentStatsDetailSchema>
export type AgentSettlement = z.infer<typeof agentSettlementSchema>

export interface AgentProfileData {
  profile: AgentProfile
  current_version?: AgentMarginVersion | null
  username: string
}

export interface AgentStatsData {
  summary: AgentStatsSummary
  details: AgentStatsDetail[]
  total: number
}

export interface AgentConfigPayload {
  enabled: boolean
  remark: string
  group_margins: Array<{
    group: string
    gross_margin_rate: number
    platform_retention_rate: number
  }>
}

export interface AgentStatsParams {
  start_timestamp: number
  end_timestamp: number
  p?: number
  page_size?: number
  keyword?: string
  group?: string
}

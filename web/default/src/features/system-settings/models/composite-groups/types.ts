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
export const COMPOSITE_OPERATIONS = {
  generation: 'image_generation',
  edit: 'image_edit',
} as const

export type CompositeOperation =
  (typeof COMPOSITE_OPERATIONS)[keyof typeof COMPOSITE_OPERATIONS]

export type CompositeGroupRoute = {
  id?: number
  composite_group_id?: number
  operation: CompositeOperation
  route_order: number
  physical_group: string
  internal_model: string
  retry_count: number
  retry_status_codes: string
  status: number
}

export type CompositeGroup = {
  id: number
  name: string
  public_model: string
  display_name: string
  description: string
  status: number
  user_selectable: boolean
  pricing_visible: boolean
  generation_enabled: boolean
  edit_enabled: boolean
  created_time: number
  updated_time: number
  routes: CompositeGroupRoute[]
}

export type CompositeRouteFormValue = {
  client_key: string
  physical_group: string
  internal_model: string
  retry_count: number
  retry_status_codes: string
}

export type CompositeGroupFormInput = {
  name: string
  public_model: string
  display_name: string
  description: string
  status: boolean
  user_selectable: boolean
  pricing_visible: boolean
  generation_enabled: boolean
  edit_enabled: boolean
  routes: CompositeRouteFormValue[]
}

export type CompositeGroupPayload = Omit<
  CompositeGroup,
  'id' | 'created_time' | 'updated_time'
>

export type CompositeGroupApiResponse<T> = {
  success: boolean
  message?: string
  data: T
}

export type CompositeGroupOptions = {
  groups: string[]
  models: Array<{
    name: string
    billingMode: 'per-request' | 'per-token' | 'tiered'
    groups: string[]
  }>
}

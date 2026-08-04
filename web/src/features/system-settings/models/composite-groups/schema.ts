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
import * as z from 'zod'

import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'

import {
  COMPOSITE_OPERATIONS,
  type CompositeGroup,
  type CompositeGroupFormInput,
  type CompositeGroupPayload,
} from './types'

const routeSchema = z.object({
  client_key: z.string(),
  physical_group: z.string().trim().min(1, 'Physical group is required'),
  internal_model: z.string().trim().min(1, 'Internal model is required'),
  retry_count: z
    .number()
    .int()
    .min(0, 'Retry count cannot be negative')
    .max(10, 'Retry count cannot exceed 10'),
  retry_status_codes: z.string().superRefine((value, ctx) => {
    const parsed = parseHttpStatusCodeRules(value)
    if (!parsed.ok) {
      ctx.addIssue({
        code: 'custom',
        message: 'Invalid retry status code rules',
      })
    }
  }),
})

export const compositeGroupSchema = z
  .object({
    name: z.string().trim().min(1, 'Composite group ID is required'),
    public_model: z.string().trim().min(1, 'Public request model is required'),
    display_name: z.string().trim(),
    description: z.string().trim(),
    status: z.boolean(),
    user_selectable: z.boolean(),
    pricing_visible: z.boolean(),
    generation_enabled: z.boolean(),
    edit_enabled: z.boolean(),
    routes: z.array(routeSchema),
  })
  .superRefine((value, ctx) => {
    if (!value.generation_enabled && !value.edit_enabled) {
      ctx.addIssue({
        code: 'custom',
        path: ['generation_enabled'],
        message: 'Enable at least one image operation',
      })
    }
    if (value.routes.length === 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['routes'],
        message: 'Add at least one route',
      })
    }
  })

export function createCompositeGroupDefaults(
  group?: CompositeGroup | null
): CompositeGroupFormInput {
  const toRoute = (route: CompositeGroup['routes'][number]) => ({
    client_key: route.id ? `route-${route.id}` : crypto.randomUUID(),
    physical_group: route.physical_group,
    internal_model: route.internal_model,
    retry_count: route.retry_count,
    retry_status_codes: route.retry_status_codes,
  })
  const generationRoutes =
    group?.routes.filter(
      (route) => route.operation === COMPOSITE_OPERATIONS.generation
    ) ?? []
  const sharedRoutes =
    generationRoutes.length > 0
      ? generationRoutes
      : (group?.routes.filter(
          (route) => route.operation === COMPOSITE_OPERATIONS.edit
        ) ?? [])
  return {
    name: group?.name ?? '',
    public_model: group?.public_model ?? '',
    display_name: group?.display_name ?? '',
    description: group?.description ?? '',
    status: group?.status === 1,
    user_selectable: group?.user_selectable ?? true,
    pricing_visible: group?.pricing_visible ?? false,
    generation_enabled: group?.generation_enabled ?? true,
    edit_enabled: group?.edit_enabled ?? true,
    routes: sharedRoutes
      .sort((a, b) => a.route_order - b.route_order)
      .map(toRoute),
  }
}

export function serializeCompositeGroup(
  input: CompositeGroupFormInput
): CompositeGroupPayload {
  const values = compositeGroupSchema.parse(input)
  const toPayloadRoute = (
    route: CompositeGroupFormInput['routes'][number],
    index: number,
    operation: CompositeGroupPayload['routes'][number]['operation']
  ) => ({
    physical_group: route.physical_group,
    internal_model: route.internal_model,
    retry_count: route.retry_count,
    retry_status_codes: parseHttpStatusCodeRules(route.retry_status_codes)
      .normalized,
    operation,
    route_order: index + 1,
    status: 1,
  })
  const routes = [] as CompositeGroupPayload['routes']
  if (values.generation_enabled) {
    routes.push(
      ...values.routes.map((route, index) =>
        toPayloadRoute(route, index, COMPOSITE_OPERATIONS.generation)
      )
    )
  }
  if (values.edit_enabled) {
    routes.push(
      ...values.routes.map((route, index) =>
        toPayloadRoute(route, index, COMPOSITE_OPERATIONS.edit)
      )
    )
  }
  return {
    name: values.name,
    public_model: values.public_model,
    display_name: values.display_name,
    description: values.description,
    status: values.status ? 1 : 0,
    user_selectable: values.user_selectable,
    pricing_visible: values.pricing_visible,
    generation_enabled: values.generation_enabled,
    edit_enabled: values.edit_enabled,
    routes,
  }
}

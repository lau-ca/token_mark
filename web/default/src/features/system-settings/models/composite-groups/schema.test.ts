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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { compositeGroupSchema, serializeCompositeGroup } from './schema'

const validInput = {
  name: 'image_stable',
  public_model: 'gpt-image-stable',
  display_name: 'Stable image',
  description: '',
  status: true,
  user_selectable: true,
  pricing_visible: false,
  generation_enabled: true,
  edit_enabled: false,
  generation_routes: [
    {
      client_key: 'route-1',
      physical_group: 'fixed_image',
      internal_model: 'gpt-image-2-w',
      retry_count: 2,
      retry_status_codes: '429, 500-599',
    },
  ],
  edit_routes: [],
}

describe('compositeGroupSchema', () => {
  test('requires the public identifiers', () => {
    const result = compositeGroupSchema.safeParse({
      ...validInput,
      name: '',
      public_model: '',
    })
    assert.equal(result.success, false)
  })

  test('requires routes for every enabled operation', () => {
    const result = compositeGroupSchema.safeParse({
      ...validInput,
      generation_routes: [],
    })
    assert.equal(result.success, false)
  })

  test('bounds retry count and validates status rules', () => {
    const result = compositeGroupSchema.safeParse({
      ...validInput,
      generation_routes: [
        {
          ...validInput.generation_routes[0],
          retry_count: 11,
          retry_status_codes: '700',
        },
      ],
    })
    assert.equal(result.success, false)
  })

  test('serializes route order and operation', () => {
    const payload = serializeCompositeGroup(validInput)
    assert.equal(payload.routes.length, 1)
    assert.equal(payload.routes[0].operation, 'image_generation')
    assert.equal(payload.routes[0].route_order, 1)
    assert.equal(payload.routes[0].retry_status_codes, '429,500-599')
  })
})

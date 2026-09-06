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

import {
  getModelCapabilities,
  parseModelCapabilityConfig,
  serializeModelCapabilityConfig,
} from '../model-capabilities'

describe('model capability config', () => {
  test('preserves unknown JSON fields when editing known capability fields', () => {
    const config = parseModelCapabilityConfig(`{
      "version": 2,
      "future": {"enabled": true},
      "endpoints": {
        "image-generation": {
          "capabilities": ["image.generate"],
          "provider_extension": {"mode": "custom"}
        }
      }
    }`)

    config.endpoints['image-generation'] = {
      ...config.endpoints['image-generation'],
      capabilities: ['image.generate', 'image.edit'],
    }

    const serialized = serializeModelCapabilityConfig(config)
    const parsed = JSON.parse(serialized) as Record<string, unknown>
    assert.deepEqual(parsed.future, { enabled: true })
    assert.deepEqual(
      (parsed.endpoints as Record<string, Record<string, unknown>>)[
        'image-generation'
      ].provider_extension,
      { mode: 'custom' }
    )
    assert.deepEqual(getModelCapabilities(serialized), [
      'image.generate',
      'image.edit',
    ])
  })
})

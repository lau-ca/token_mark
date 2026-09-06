/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

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

import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

describe('image response URL prefix channel setting', () => {
  test('hydrates the configured prefix from channel JSON', () => {
    const defaults = transformChannelToFormDefaults({
      name: 'OpenAI image upstream',
      type: 1,
      settings: JSON.stringify({
        image_response_url_prefix: 'https://trusted.example/images/',
      }),
      channel_info: { multi_key_mode: 'random' },
    } as Channel)

    assert.equal(
      defaults.image_response_url_prefix,
      'https://trusted.example/images/'
    )
  })

  test('trims and serializes the prefix for create and update', () => {
    const form = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'OpenAI image upstream',
      type: 1,
      key: 'test-key',
      models: 'gpt-image-2',
      force_image_b64_json_no_url: false,
      image_response_url_prefix: '  https://trusted.example/images/  ',
    }

    const createPayload = transformFormDataToCreatePayload(form)
    const updatePayload = transformFormDataToUpdatePayload(form, 7)

    assert.equal(
      JSON.parse(String(createPayload.channel.settings))
        .image_response_url_prefix,
      'https://trusted.example/images/'
    )
    assert.equal(
      JSON.parse(String(updatePayload.settings)).image_response_url_prefix,
      'https://trusted.example/images/'
    )
  })

  test('keeps an empty prefix as unrestricted', () => {
    const form = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'OpenAI image upstream',
      type: 1,
      key: 'test-key',
      models: 'gpt-image-2',
      image_response_url_prefix: '   ',
    }

    const payload = transformFormDataToCreatePayload(form)

    assert.equal(
      JSON.parse(String(payload.channel.settings)).image_response_url_prefix,
      ''
    )
  })
})

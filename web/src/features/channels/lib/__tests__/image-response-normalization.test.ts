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

import { CHANNEL_TYPE_NEW_API } from '../../constants'
import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

function imageChannelForm(type = 1) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'OpenAI image upstream',
    type,
    key: 'test-key',
    models: 'gpt-image-2',
    normalize_openai_image_response: true,
  }
}

describe('OpenAI image response normalization channel setting', () => {
  test('hydrates the enabled setting from channel JSON', () => {
    const defaults = transformChannelToFormDefaults({
      name: 'OpenAI image upstream',
      type: 1,
      settings: JSON.stringify({ normalize_openai_image_response: true }),
      channel_info: { multi_key_mode: 'random' },
    } as Channel)

    assert.equal(defaults.normalize_openai_image_response, true)
  })

  test('serializes the setting for supported OpenAI-compatible channels', () => {
    for (const type of [
      1,
      3,
      6,
      7,
      8,
      9,
      10,
      12,
      13,
      19,
      20,
      22,
      31,
      47,
      48,
      58,
      59,
      CHANNEL_TYPE_NEW_API,
    ]) {
      const createPayload = transformFormDataToCreatePayload(
        imageChannelForm(type)
      )
      const updatePayload = transformFormDataToUpdatePayload(
        imageChannelForm(type),
        7
      )

      assert.equal(
        JSON.parse(String(createPayload.channel.settings))
          .normalize_openai_image_response,
        true
      )
      assert.equal(
        JSON.parse(String(updatePayload.settings))
          .normalize_openai_image_response,
        true
      )
    }
  })

  test('removes the setting for unrelated channel types', () => {
    const payload = transformFormDataToCreatePayload({
      ...imageChannelForm(),
      type: 57,
      settings: JSON.stringify({ normalize_openai_image_response: true }),
    })

    assert.equal(
      'normalize_openai_image_response' in
        JSON.parse(String(payload.channel.settings)),
      false
    )
  })
})

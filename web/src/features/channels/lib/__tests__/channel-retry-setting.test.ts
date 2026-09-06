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

import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  MAX_CHANNEL_RETRY_TIMES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

function retryChannelForm(retryTimes: number) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Retrying upstream',
    key: 'test-key',
    models: 'gpt-5.4',
    retry_times: retryTimes,
  }
}

describe('channel retry setting', () => {
  test('defaults legacy channels to zero retries', () => {
    const defaults = transformChannelToFormDefaults({
      name: 'Legacy upstream',
      type: 1,
      setting: '{}',
      settings: '{}',
      channel_info: { multi_key_mode: 'random' },
    } as Channel)

    assert.equal(defaults.retry_times, 0)
  })

  test('hydrates and serializes the configured retry count', () => {
    const defaults = transformChannelToFormDefaults({
      name: 'Retrying upstream',
      type: 1,
      setting: JSON.stringify({ retry_times: 2 }),
      settings: '{}',
      channel_info: { multi_key_mode: 'random' },
    } as Channel)
    const createPayload = transformFormDataToCreatePayload(retryChannelForm(2))
    const updatePayload = transformFormDataToUpdatePayload(
      retryChannelForm(2),
      7
    )

    assert.equal(defaults.retry_times, 2)
    assert.equal(
      JSON.parse(String(createPayload.channel.setting)).retry_times,
      2
    )
    assert.equal(JSON.parse(String(updatePayload.setting)).retry_times, 2)
  })

  test('rejects retry counts above the administrator limit', () => {
    const result = channelFormSchema.safeParse(
      retryChannelForm(MAX_CHANNEL_RETRY_TIMES + 1)
    )

    assert.equal(result.success, false)
  })
})

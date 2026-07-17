import { describe, expect, test } from 'bun:test'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from './channel-form.ts'

describe('channel balance form transforms', () => {
  test('includes New API account settings when creating a channel', () => {
    const payload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'new-api-upstream',
      key: 'sk-channel',
      models: 'gpt-5',
      balance_platform: 'new_api',
      balance_base_url: 'https://example.com///',
      balance_user_id: 1787,
      balance_auth_key: 'account-token',
    })

    expect(payload.channel).toMatchObject({
      balance_platform: 'new_api',
      balance_base_url: 'https://example.com',
      balance_user_id: 1787,
      balance_auth_key: 'account-token',
    })
  })

  test('omits an unchanged account token when editing', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'new-api-upstream',
        models: 'gpt-5',
        balance_platform: 'new_api',
        balance_base_url: 'https://example.com/',
        balance_user_id: 1787,
        balance_auth_key: '',
      },
      10
    )

    expect(payload).toMatchObject({
      balance_platform: 'new_api',
      balance_base_url: 'https://example.com',
      balance_user_id: 1787,
    })
    expect(payload).not.toHaveProperty('balance_auth_key')
  })

  test('Sub2API reuses the channel key and clears New API user settings', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'sub2api-upstream',
        models: 'gpt-image-2',
        balance_platform: 'sub2api',
        balance_base_url: 'https://img-api.example.com/',
        balance_user_id: 1787,
        balance_auth_key: 'unused-token',
      },
      11
    )

    expect(payload).toMatchObject({
      balance_platform: 'sub2api',
      balance_base_url: 'https://img-api.example.com',
      balance_user_id: 0,
    })
    expect(payload).not.toHaveProperty('balance_auth_key')
  })
})

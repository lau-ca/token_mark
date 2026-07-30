import { describe, expect, test } from 'bun:test'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
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

describe('image prompt parameter append setting', () => {
  test('serializes enabled channel configuration', () => {
    const payload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'image-upstream',
      key: 'sk-channel',
      models: 'gpt-image-2',
      image_prompt_parameter_append_enabled: true,
      image_prompt_parameter_append_models:
        'gpt-image-2, gpt-image-2, gpt-image-2-custom',
      image_prompt_parameter_append_template:
        'Render size={{size}} quality={{quality}}.',
    })

    expect(JSON.parse(payload.channel.setting)).toMatchObject({
      image_prompt_parameter_append: {
        enabled: true,
        models: ['gpt-image-2', 'gpt-image-2-custom'],
        template: 'Render size={{size}} quality={{quality}}.',
      },
    })
  })

  test('hydrates existing channel configuration', () => {
    const defaults = transformChannelToFormDefaults({
      id: 12,
      name: 'image-upstream',
      type: 1,
      status: 1,
      setting: JSON.stringify({
        image_prompt_parameter_append: {
          enabled: true,
          models: ['gpt-image-2', 'gpt-image-2-custom'],
          template: 'Render size={{size}} quality={{quality}}.',
        },
      }),
      channel_info: {},
    })

    expect(defaults).toMatchObject({
      image_prompt_parameter_append_enabled: true,
      image_prompt_parameter_append_models: 'gpt-image-2, gpt-image-2-custom',
      image_prompt_parameter_append_template:
        'Render size={{size}} quality={{quality}}.',
    })
  })
})

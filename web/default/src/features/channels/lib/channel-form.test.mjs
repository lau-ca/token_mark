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

describe('image prompt parameter override migration', () => {
  test('migrates existing channel configuration into parameter operations', () => {
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
      param_override: JSON.stringify({
        operations: [{ path: 'temperature', mode: 'set', value: 0.7 }],
      }),
      channel_info: {},
    })

    expect(JSON.parse(defaults.param_override)).toEqual({
      operations: [
        { path: 'temperature', mode: 'set', value: 0.7 },
        {
          description: 'Append GPT Image size and quality to prompt',
          phase: 'request',
          path: 'prompt',
          mode: 'append_template',
          value: '\n\nRender size=${body.size} quality=${body.quality}.',
          conditions: [
            { path: 'model', mode: 'full', value: 'gpt-image-2' },
            {
              path: 'model',
              mode: 'full',
              value: 'gpt-image-2-custom',
            },
          ],
          logic: 'OR',
        },
      ],
    })

    const payload = transformFormDataToUpdatePayload(defaults, 12)
    expect(JSON.parse(payload.setting)).not.toHaveProperty(
      'image_prompt_parameter_append'
    )
    expect(JSON.parse(payload.param_override).operations).toHaveLength(2)
  })

  test('does not duplicate an existing request template operation', () => {
    const existingOperation = {
      phase: 'request',
      path: 'prompt',
      mode: 'append_template',
      value:
        '\n\nOutput image requirements: size=${body.size}; quality=${body.quality}.',
    }
    const defaults = transformChannelToFormDefaults({
      id: 13,
      name: 'image-upstream',
      type: 1,
      status: 1,
      setting: JSON.stringify({
        image_prompt_parameter_append: { enabled: true },
      }),
      param_override: JSON.stringify({ operations: [existingOperation] }),
      channel_info: {},
    })

    expect(JSON.parse(defaults.param_override).operations).toEqual([
      existingOperation,
    ])
  })
})

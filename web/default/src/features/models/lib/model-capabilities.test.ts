import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getEndpointDefinition,
  parseModelEndpointDefinitions,
} from './model-capabilities'

describe('model capability configuration', () => {
  test('legacy endpoint arrays do not inject playground configuration', () => {
    const endpoints = parseModelEndpointDefinitions(
      JSON.stringify(['openai', 'image-generation'])
    )

    assert.deepEqual(endpoints, {
      openai: {},
      'image-generation': {},
    })
  })

  test('administrator integration remains attached to the endpoint', () => {
    const endpoints = parseModelEndpointDefinitions(
      JSON.stringify({
        'openai-video': {
          playground: {
            integration: {
              overview: 'Administrator guide',
              interfaces: [
                {
                  key: 'create',
                  title: 'Create video',
                  method: 'POST',
                  path: '/v1/videos',
                  curl_template: 'custom curl',
                },
              ],
            },
          },
        },
      })
    )

    assert.equal(
      getEndpointDefinition(endpoints, 'openai-video').playground?.integration
        ?.overview,
      'Administrator guide'
    )
  })
})

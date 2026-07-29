import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PlaygroundIntegrationDefinition } from '../../types'
import {
  resolveIntegrationTemplate,
  resolvePlaygroundIntegrationGuide,
  type PlaygroundIntegrationVariables,
} from './playground-integration-guide'

const variables: PlaygroundIntegrationVariables = {
  api_key: '$NEW_API_KEY',
  base_url: 'https://api.frimodel.com',
  group: 'default',
  model: 'seedance2.0',
  parameters_json: '{}',
  prompt: 'demo',
  reference_url: 'https://example.com/image.png',
  request_curl: 'curl create',
  request_json: '{"model":"seedance2.0"}',
  task_id: '$TASK_ID',
}

const videoDefinition: PlaygroundIntegrationDefinition = {
  overview: 'Create a task, then query it.',
  interfaces: [
    {
      key: 'create',
      title: 'Create video',
      method: 'POST',
      path: '/v1/videos',
      curl_template: String.raw`curl '{{base_url}}/v1/videos' \
  -H 'Authorization: Bearer {{api_key}}' \
  -H 'Content-Type: application/json' \
  -d '{{request_json}}'`,
    },
    {
      key: 'query',
      title: 'Query video',
      method: 'GET',
      path: '/v1/videos/{{task_id}}',
      curl_template: String.raw`curl '{{base_url}}/v1/videos/{{task_id}}' \
  -H 'Authorization: Bearer {{api_key}}'`,
    },
  ],
  complete_example: '# 1. Create task\n# 2. Query task',
}

describe('playground integration guide', () => {
  test('administrator video guide resolves create and query interfaces', () => {
    const guide = resolvePlaygroundIntegrationGuide({
      apiBaseUrl: 'https://api.frimodel.com',
      definition: videoDefinition,
      model: 'seedance2.0',
      group: 'Seedance2.0',
      prompt: 'A slow camera move',
      parameters: { seconds: 15, size: '1280x720' },
      requestCurl: 'curl create',
      requestJson: JSON.stringify({
        model: 'seedance2.0',
        group: 'Seedance2.0',
        prompt: 'A slow camera move',
        seconds: 15,
        size: '1280x720',
      }),
    })

    assert.deepEqual(
      guide.interfaces.map((item) => item.key),
      ['create', 'query']
    )
    assert.match(guide.interfaces[0].curl, /seedance2\.0/)
    assert.match(guide.interfaces[1].curl, /\$TASK_ID/)
  })

  test('unknown variables remain visible', () => {
    assert.equal(
      resolveIntegrationTemplate('{{unknown}}', variables),
      '{{unknown}}'
    )
  })

  test('request JSON remains safe inside single-quoted curl data', () => {
    const guide = resolvePlaygroundIntegrationGuide({
      apiBaseUrl: 'https://api.frimodel.com',
      definition: videoDefinition,
      model: 'seedance2.0',
      group: 'default',
      prompt: "director's cut",
      parameters: {},
      requestCurl: 'curl create',
      requestJson: JSON.stringify({ prompt: "director's cut" }),
    })

    assert.match(guide.interfaces[0].curl, /director'"'"'s cut/)
  })
})

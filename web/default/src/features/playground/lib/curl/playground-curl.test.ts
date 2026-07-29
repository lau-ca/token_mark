import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type {
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundParameterOption,
} from '../../types'
import {
  buildPlaygroundChatCurl,
  buildPlaygroundMediaCurl,
  resolvePlaygroundApiBaseUrl,
} from './playground-curl'

const config: PlaygroundConfig = {
  model: 'gpt-4.1',
  group: 'default',
  mode_selections: {
    chat: { model: 'gpt-4.1', group: 'default' },
    image: { model: '', group: 'default' },
    video: { model: '', group: 'default' },
  },
  temperature: 0.2,
  top_p: 0.9,
  max_tokens: 2048,
  frequency_penalty: 0,
  presence_penalty: 0,
  seed: 7,
  stream: true,
}

const parameterEnabled: ParameterEnabled = {
  temperature: true,
  top_p: false,
  max_tokens: true,
  frequency_penalty: false,
  presence_penalty: false,
  seed: true,
}

const parameters: PlaygroundParameterOption[] = [
  { key: 'size', type: 'enum', options: ['1024x1024', '2048x2048'] },
  {
    key: 'quality',
    request_path: 'extra_body.google.image_config.quality',
    type: 'enum',
    options: ['standard', 'high'],
  },
]

describe('playground curl builder', () => {
  test('builds chat curl from the current draft and enabled parameters', () => {
    const curl = buildPlaygroundChatCurl({
      apiBaseUrl: 'https://api.example.com/',
      config,
      parameterEnabled,
      prompt: "Explain Newton's laws",
    })

    assert.match(
      curl,
      /^curl 'https:\/\/api\.example\.com\/v1\/chat\/completions'/
    )
    assert.match(curl, /-H "Authorization: Bearer \$NEW_API_KEY"/)
    assert.match(curl, /"content": "Explain Newton'"'"'s laws"/)
    assert.match(curl, /"temperature": 0\.2/)
    assert.match(curl, /"max_tokens": 2048/)
    assert.match(curl, /"seed": 7/)
    assert.doesNotMatch(curl, /"top_p"/)
  })

  test('builds JSON image generation curl with administrator-defined paths', () => {
    const curl = buildPlaygroundMediaCurl({
      apiBaseUrl: 'https://api.example.com',
      endpointType: 'image-generation',
      group: 'vip',
      mode: 'image',
      model: 'gpt-image-1',
      parameters,
      prompt: 'A quiet lake',
      values: { size: '2048x2048', quality: 'high' },
    })

    assert.match(curl, /\/v1\/images\/generations'/)
    assert.match(curl, /"group": "vip"/)
    assert.match(curl, /"size": "2048x2048"/)
    assert.match(curl, /"extra_body": \{/)
    assert.match(curl, /"quality": "high"/)
  })

  test('builds multipart image editing curl with a local file placeholder', () => {
    const curl = buildPlaygroundMediaCurl({
      apiBaseUrl: 'https://api.example.com',
      canEditImage: true,
      endpointType: 'image-generation',
      group: 'default',
      hasReferenceImage: true,
      mode: 'image',
      model: 'gpt-image-1',
      parameters,
      prompt: 'Remove the sign',
      values: { size: '1024x1024' },
    })

    assert.match(curl, /\/v1\/images\/edits'/)
    assert.match(curl, /-F 'image=@\/path\/to\/reference\.png'/)
    assert.match(curl, /-F 'size=1024x1024'/)
    assert.doesNotMatch(curl, /Content-Type: application\/json/)
  })

  test('builds Gemini image curl with an image data placeholder', () => {
    const curl = buildPlaygroundMediaCurl({
      apiBaseUrl: 'https://api.example.com',
      canEditImage: true,
      endpointType: 'gemini',
      group: 'default',
      hasReferenceImage: true,
      mode: 'image',
      model: 'gemini-3.1-flash-image-preview',
      parameters,
      prompt: 'Restyle this image',
      values: { quality: 'high' },
    })

    assert.match(curl, /\/v1\/chat\/completions'/)
    assert.match(curl, /"type": "image_url"/)
    assert.match(curl, /data:image\/png;base64,<BASE64_IMAGE>/)
    assert.match(curl, /"stream": false/)
    assert.doesNotMatch(curl, /"prompt"/)
  })

  test('builds video curl with dynamic parameters and reference URL', () => {
    const curl = buildPlaygroundMediaCurl({
      apiBaseUrl: 'https://api.example.com',
      canGenerateVideoFromImage: true,
      endpointType: 'openai-video',
      group: 'default',
      mode: 'video',
      model: 'sora-2',
      parameters: [
        { key: 'seconds', type: 'number' },
        { key: 'resolution', type: 'enum', options: ['720p', '1080p'] },
      ],
      prompt: 'A slow camera move',
      referenceUrl: 'https://cdn.example.com/reference.png',
      values: { seconds: 8, resolution: '1080p' },
    })

    assert.match(curl, /\/v1\/videos'/)
    assert.match(curl, /"seconds": 8/)
    assert.match(curl, /"resolution": "1080p"/)
    assert.match(curl, /"image": "https:\/\/cdn\.example\.com\/reference\.png"/)
  })

  test('prefers the configured public API base URL', () => {
    assert.equal(
      resolvePlaygroundApiBaseUrl(
        {
          public_api_base_url: 'https://public.example.com/',
          server_address: 'https://internal.example.com',
        },
        'https://page.example.com'
      ),
      'https://public.example.com'
    )
  })
})

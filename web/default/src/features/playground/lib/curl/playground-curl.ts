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

import type { SystemStatus } from '@/features/auth/types'

import type {
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundMode,
  PlaygroundParameterOption,
} from '../../types'
import { buildPlaygroundMediaPayload } from '../media/playground-media-request'
import { buildChatCompletionPayload } from '../streaming/payload-builder'

type BuildPlaygroundChatCurlOptions = {
  apiBaseUrl: string
  config: PlaygroundConfig
  parameterEnabled: ParameterEnabled
  prompt: string
}

type BuildPlaygroundMediaCurlOptions = {
  apiBaseUrl: string
  canEditImage?: boolean
  canGenerateVideoFromImage?: boolean
  endpointType: string
  group: string
  hasReferenceImage?: boolean
  mode: Exclude<PlaygroundMode, 'chat'>
  model: string
  parameters: PlaygroundParameterOption[]
  prompt: string
  referenceUrl?: string
  values: Record<string, string | number | boolean>
}

function normalizeApiBaseUrl(apiBaseUrl: string): string {
  return apiBaseUrl.trim().replace(/\/+$/, '')
}

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'"'"'`)}'`
}

function buildJsonCurl(
  apiBaseUrl: string,
  path: string,
  payload: Record<string, unknown>
): string {
  const url = `${normalizeApiBaseUrl(apiBaseUrl)}${path}`
  return [
    `curl ${shellQuote(url)} \\`,
    '  -H "Authorization: Bearer $NEW_API_KEY" \\',
    `  -H 'Content-Type: application/json' \\`,
    `  -d ${shellQuote(JSON.stringify(payload, null, 2))}`,
  ].join('\n')
}

export function resolvePlaygroundApiBaseUrl(
  status: SystemStatus | null,
  fallbackOrigin = typeof window === 'undefined' ? '' : window.location.origin
): string {
  const candidates = [
    status?.public_api_base_url,
    status?.data?.public_api_base_url,
    status?.server_address,
    status?.data?.server_address,
    fallbackOrigin,
  ]

  for (const candidate of candidates) {
    if (typeof candidate === 'string' && candidate.trim()) {
      return normalizeApiBaseUrl(candidate)
    }
  }

  return ''
}

export function buildPlaygroundChatCurl(
  options: BuildPlaygroundChatCurlOptions
): string {
  const prompt = options.prompt.trim()
  if (!options.apiBaseUrl || !options.config.model || !prompt) return ''

  const payload = buildChatCompletionPayload(
    [
      {
        key: 'curl-draft',
        from: 'user',
        versions: [{ id: 'curl-draft', content: prompt }],
      },
    ],
    options.config,
    options.parameterEnabled
  )

  return buildJsonCurl(
    options.apiBaseUrl,
    '/v1/chat/completions',
    payload as unknown as Record<string, unknown>
  )
}

function buildImageEditCurl(options: BuildPlaygroundMediaCurlOptions): string {
  const url = `${normalizeApiBaseUrl(options.apiBaseUrl)}/v1/images/edits`
  const fields = [
    ['model', options.model],
    ['group', options.group],
    ['prompt', options.prompt.trim()],
    ['image', '@/path/to/reference.png'],
  ]

  for (const parameter of options.parameters) {
    const value = options.values[parameter.key]
    if (value === undefined || value === '') continue
    fields.push([parameter.key, String(value)])
  }

  const lines = [
    `curl ${shellQuote(url)} \\`,
    '  -H "Authorization: Bearer $NEW_API_KEY" \\',
  ]
  fields.forEach(([key, value], index) => {
    const suffix = index === fields.length - 1 ? '' : ' \\'
    lines.push(`  -F ${shellQuote(`${key}=${value}`)}${suffix}`)
  })
  return lines.join('\n')
}

export function buildPlaygroundMediaCurl(
  options: BuildPlaygroundMediaCurlOptions
): string {
  const prompt = options.prompt.trim()
  if (!options.apiBaseUrl || !options.model || !prompt) return ''

  const payload = buildPlaygroundMediaPayload({
    group: options.group,
    model: options.model,
    parameters: options.parameters,
    prompt,
    values: options.values,
  })

  if (options.mode === 'video') {
    const referenceUrl = options.referenceUrl?.trim()
    if (referenceUrl && options.canGenerateVideoFromImage) {
      payload.image = referenceUrl
    }
    return buildJsonCurl(options.apiBaseUrl, '/v1/videos', payload)
  }

  if (options.endpointType === 'gemini') {
    delete payload.prompt
    const content: unknown =
      options.hasReferenceImage && options.canEditImage
        ? [
            { type: 'text', text: prompt },
            {
              type: 'image_url',
              image_url: {
                url: 'data:image/png;base64,<BASE64_IMAGE>',
              },
            },
          ]
        : prompt
    payload.messages = [{ role: 'user', content }]
    payload.stream = false
    return buildJsonCurl(options.apiBaseUrl, '/v1/chat/completions', payload)
  }

  if (options.hasReferenceImage && options.canEditImage) {
    return buildImageEditCurl(options)
  }

  return buildJsonCurl(options.apiBaseUrl, '/v1/images/generations', payload)
}

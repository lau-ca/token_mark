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

import type { PlaygroundMedia } from '../../types'

export type ImageResult = {
  content: string
  media: PlaygroundMedia[]
}

export function setMediaRequestValue(
  target: Record<string, unknown>,
  path: string,
  value: unknown
) {
  const parts = path.split('.').filter(Boolean)
  if (
    parts.length === 0 ||
    parts.some((part) =>
      ['__proto__', 'prototype', 'constructor'].includes(part)
    )
  ) {
    return
  }

  let current = target
  for (let index = 0; index < parts.length - 1; index++) {
    const part = parts[index]
    const next = current[part]
    if (!next || typeof next !== 'object' || Array.isArray(next)) {
      current[part] = {}
    }
    current = current[part] as Record<string, unknown>
  }
  const finalPart = parts.at(-1)
  if (finalPart) {
    current[finalPart] = value
  }
}

function getBase64ImageMimeType(data: string): string {
  if (data.startsWith('/9j/')) return 'image/jpeg'
  if (data.startsWith('UklGR')) return 'image/webp'
  return 'image/png'
}

export function getImageResult(response: Record<string, unknown>): ImageResult {
  const data = Array.isArray(response.data) ? response.data : []
  const media = data.flatMap((item): PlaygroundMedia[] => {
    if (!item || typeof item !== 'object') return []
    const record = item as Record<string, unknown>
    for (const key of ['url', 'image_url', 'provider_image_url']) {
      if (typeof record[key] === 'string') {
        return [{ type: 'image', url: record[key] }]
      }
    }
    if (typeof record.b64_json === 'string') {
      const mimeType = getBase64ImageMimeType(record.b64_json)
      return [
        {
          type: 'image',
          url: `data:${mimeType};base64,${record.b64_json}`,
          isTransient: true,
        },
      ]
    }
    return []
  })
  if (media.length > 0) {
    return { content: 'Image generation completed', media }
  }

  const choices = Array.isArray(response.choices) ? response.choices : []
  const first = choices[0] as Record<string, unknown> | undefined
  const message = first?.message as Record<string, unknown> | undefined
  const content = typeof message?.content === 'string' ? message.content : ''
  const imagePattern = /!\[[^\]]*\]\((data:image\/[^)]+|https?:\/\/[^)]+)\)/g
  const markdownImages = [...content.matchAll(imagePattern)]
  if (markdownImages.length === 0) {
    return { content, media: [] }
  }

  return {
    content:
      content.replaceAll(imagePattern, '').trim() ||
      'Image generation completed',
    media: markdownImages.map((match) => {
      let url = match[1]
      if (url.startsWith('data:image/') && url.includes(';base64,')) {
        const base64Data = url.slice(url.indexOf(',') + 1)
        url = `data:${getBase64ImageMimeType(base64Data)};base64,${base64Data}`
      }

      return {
        type: 'image',
        url,
        isTransient: url.startsWith('data:'),
      }
    }),
  }
}

export function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener('error', () => {
      reject(reader.error ?? new Error())
    })
    reader.addEventListener('load', () => {
      resolve(String(reader.result ?? ''))
    })
    reader.readAsDataURL(file)
  })
}

export function unwrapMediaResponse(
  value: Record<string, unknown>
): Record<string, unknown> {
  return value.data && typeof value.data === 'object'
    ? (value.data as Record<string, unknown>)
    : value
}

export function isCanceledMediaRequest(error: unknown): boolean {
  if (error instanceof DOMException && error.name === 'AbortError') {
    return true
  }
  if (!error || typeof error !== 'object') return false
  return (
    (error as { code?: string }).code === 'ERR_CANCELED' ||
    (error as { name?: string }).name === 'CanceledError'
  )
}

export async function downloadPlaygroundMedia(
  url: string,
  filename: string
): Promise<void> {
  const response = await fetch(url, { credentials: 'include' })
  if (!response.ok) {
    throw new Error(`Download failed with status ${response.status}`)
  }

  const blobUrl = URL.createObjectURL(await response.blob())
  const anchor = document.createElement('a')
  anchor.href = blobUrl
  anchor.download = filename
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(blobUrl), 0)
}

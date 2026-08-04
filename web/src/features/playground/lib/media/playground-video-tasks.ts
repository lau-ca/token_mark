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

import { api } from '@/lib/api'

import { API_ENDPOINTS } from '../../constants'
import type { Message } from '../../types'
import { unwrapMediaResponse } from './playground-media-utils'

const VIDEO_POLL_INTERVAL_MS = 5000
const VIDEO_POLL_ATTEMPTS = 180

export type PlaygroundVideoTaskResult = {
  contentUrl: string
  progress: number
}

function waitForNextPoll(signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const handleAbort = () => {
      window.clearTimeout(timeout)
      reject(new DOMException('Aborted', 'AbortError'))
    }
    const timeout = window.setTimeout(() => {
      signal.removeEventListener('abort', handleAbort)
      resolve()
    }, VIDEO_POLL_INTERVAL_MS)
    signal.addEventListener('abort', handleAbort, { once: true })
  })
}

export function getPlaygroundVideoContentUrl(taskId: string): string {
  return `${API_ENDPOINTS.VIDEOS}/${taskId}/content`
}

export function getPendingVideoTaskId(message: Message): string | undefined {
  if (message.mode !== 'video' || message.status === 'complete') {
    return undefined
  }
  return message.media?.find(
    (item) => item.type === 'video' && item.taskId && !item.url
  )?.taskId
}

export async function pollPlaygroundVideoTask(
  taskId: string,
  signal: AbortSignal,
  onProgress?: (progress: number) => void
): Promise<PlaygroundVideoTaskResult> {
  for (let attempt = 0; attempt < VIDEO_POLL_ATTEMPTS; attempt++) {
    const response = await api.get(`${API_ENDPOINTS.VIDEOS}/${taskId}`, {
      skipErrorHandler: true,
      signal,
    } as Record<string, unknown>)
    const task = unwrapMediaResponse(response.data as Record<string, unknown>)
    const parsedProgress = Number(String(task.progress ?? 0).replace('%', ''))
    const progress = Number.isFinite(parsedProgress) ? parsedProgress : 0
    onProgress?.(progress)

    const status = String(task.status ?? '').toLowerCase()
    if (status === 'completed' || status === 'success') {
      return {
        contentUrl: getPlaygroundVideoContentUrl(taskId),
        progress: 100,
      }
    }
    if (status === 'failed' || status === 'failure') {
      const taskError = task.error as Record<string, unknown> | undefined
      throw new Error(String(taskError?.message ?? 'Video generation failed'))
    }
    await waitForNextPoll(signal)
  }

  throw new Error('Video generation timed out')
}

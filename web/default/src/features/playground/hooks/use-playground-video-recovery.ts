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

import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { ERROR_MESSAGES } from '../constants'
import {
  completeAssistantMessage,
  completeAssistantMessageWithError,
  getPendingVideoTaskId,
  isCanceledMediaRequest,
  pollPlaygroundVideoTask,
  updateAssistantMessageByKey,
  updateCurrentVersionContent,
} from '../lib'
import type { Message } from '../types'

type Props = {
  isLoadingMessages: boolean
  messages: Message[]
  updateMessages: (updater: (messages: Message[]) => Message[]) => void
}

export function usePlaygroundVideoRecovery(props: Props) {
  const { t } = useTranslation()
  const startedTaskIdsRef = useRef<Set<string>>(new Set())
  const controllersRef = useRef<Map<string, AbortController>>(new Map())

  useEffect(() => {
    if (props.isLoadingMessages) return

    for (const message of props.messages) {
      const taskId = getPendingVideoTaskId(message)
      if (!taskId || startedTaskIdsRef.current.has(taskId)) continue

      const controller = new AbortController()
      startedTaskIdsRef.current.add(taskId)
      controllersRef.current.set(taskId, controller)
      void pollPlaygroundVideoTask(taskId, controller.signal, (progress) => {
        props.updateMessages((messages) =>
          updateAssistantMessageByKey(messages, message.key, (current) => ({
            ...current,
            media: current.media?.map((item) =>
              item.taskId === taskId ? { ...item, progress } : item
            ),
          }))
        )
      })
        .then((result) => {
          props.updateMessages((messages) =>
            updateAssistantMessageByKey(messages, message.key, (current) =>
              completeAssistantMessage({
                ...updateCurrentVersionContent(
                  current,
                  t('Video generation completed')
                ),
                media: [
                  {
                    type: 'video',
                    url: result.contentUrl,
                    taskId,
                    filename: `generated-video-${taskId}.mp4`,
                    progress: 100,
                  },
                ],
              })
            )
          )
        })
        .catch((error: unknown) => {
          if (isCanceledMediaRequest(error)) return
          const status = (error as { response?: { status?: number } }).response
            ?.status
          if (status === 401 || status === 403) return

          const messageText =
            error instanceof Error && error.message
              ? error.message
              : t('Video generation failed')
          props.updateMessages((messages) =>
            updateAssistantMessageByKey(messages, message.key, (current) =>
              completeAssistantMessageWithError(
                current,
                messageText,
                undefined,
                t(ERROR_MESSAGES.API_REQUEST_ERROR)
              )
            )
          )
        })
        .finally(() => {
          controllersRef.current.delete(taskId)
        })
    }
  }, [props, t])

  useEffect(
    () => () => {
      for (const controller of controllersRef.current.values()) {
        controller.abort()
      }
    },
    []
  )
}

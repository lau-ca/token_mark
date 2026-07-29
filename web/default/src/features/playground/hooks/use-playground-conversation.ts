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
import { useCallback, useMemo, useState } from 'react'

import {
  appendUserMessagePair,
  applyMessageEdit,
  createRegeneratedMessages,
  removeMessageByKey,
} from '../lib'
import type {
  Message,
  PlaygroundMode,
  PlaygroundRequestContext,
} from '../types'

type UsePlaygroundConversationOptions = {
  messages: Message[]
  updateMessages: (
    updater: Message[] | ((prev: Message[]) => Message[])
  ) => void
  sendChat: (messages: Message[]) => void
  mode?: PlaygroundMode
  requestContext?: PlaygroundRequestContext
}

function attachRequestContext(
  messages: Message[],
  requestContext?: PlaygroundRequestContext
): Message[] {
  const lastMessage = messages.at(-1)
  if (!requestContext || lastMessage?.from !== 'assistant') {
    return messages
  }

  return [
    ...messages.slice(0, -1),
    {
      ...lastMessage,
      requestContext,
    },
  ]
}

export function usePlaygroundConversation({
  messages,
  updateMessages,
  sendChat,
  mode = 'chat',
  requestContext,
}: UsePlaygroundConversationOptions) {
  const [editingMessageKey, setEditingMessageKey] = useState<string | null>(
    null
  )
  const scopedMessages = useMemo(
    () => messages.filter((message) => (message.mode ?? 'chat') === mode),
    [messages, mode]
  )
  const replaceScopedMessages = useCallback(
    (nextMessages: Message[]) => {
      updateMessages((current) => [
        ...current.filter((message) => (message.mode ?? 'chat') !== mode),
        ...nextMessages,
      ])
    },
    [mode, updateMessages]
  )

  const handleSendMessage = useCallback(
    (text: string) => {
      const nextMessages = attachRequestContext(
        appendUserMessagePair(scopedMessages, text),
        requestContext
      )
      replaceScopedMessages(nextMessages)
      sendChat(nextMessages)
    },
    [replaceScopedMessages, requestContext, scopedMessages, sendChat]
  )

  const handleRegenerateMessage = useCallback(
    (message: Message) => {
      const regeneratedMessages = createRegeneratedMessages(
        scopedMessages,
        message.key
      )
      if (!regeneratedMessages) return

      const nextMessages = attachRequestContext(
        regeneratedMessages,
        requestContext
      )

      replaceScopedMessages(nextMessages)
      sendChat(nextMessages)
    },
    [replaceScopedMessages, requestContext, scopedMessages, sendChat]
  )

  const handleEditMessage = useCallback((message: Message) => {
    setEditingMessageKey(message.key)
  }, [])

  const handleEditOpenChange = useCallback((open: boolean) => {
    if (!open) {
      setEditingMessageKey(null)
    }
  }, [])

  const applyEdit = useCallback(
    (newContent: string, shouldSubmit: boolean) => {
      if (!editingMessageKey) return

      const editResult = applyMessageEdit(
        scopedMessages,
        editingMessageKey,
        newContent,
        shouldSubmit
      )
      if (!editResult) return

      setEditingMessageKey(null)
      const nextMessages = editResult.shouldSend
        ? attachRequestContext(editResult.messages, requestContext)
        : editResult.messages
      replaceScopedMessages(nextMessages)

      if (editResult.shouldSend) {
        sendChat(nextMessages)
      }
    },
    [
      editingMessageKey,
      replaceScopedMessages,
      requestContext,
      scopedMessages,
      sendChat,
    ]
  )

  const handleDeleteMessage = useCallback(
    (message: Message) => {
      updateMessages((previousMessages) =>
        removeMessageByKey(previousMessages, message.key)
      )
    },
    [updateMessages]
  )

  return {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  }
}

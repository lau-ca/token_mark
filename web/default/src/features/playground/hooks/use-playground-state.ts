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
import { useCallback, useEffect, useRef, useState } from 'react'

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../constants'
import {
  saveConfig,
  saveParameterEnabled,
  saveMessages,
  applyMessageStateUpdate,
  deleteStoredMedia,
  getStoredMediaKeys,
  getInitialParameterEnabled,
  getInitialPlaygroundConfig,
  hydrateStoredMedia,
  loadMessages,
  pruneStoredMedia,
  type MessageStateUpdater,
} from '../lib'
import type {
  Message,
  PlaygroundConfig,
  ParameterEnabled,
  ModelOption,
  GroupOption,
  PlaygroundMode,
  PlaygroundModeSelection,
} from '../types'

const MESSAGE_SAVE_DEBOUNCE_MS = 500

/**
 * Main state management hook for playground
 */
export function usePlaygroundState() {
  // Load initial state from localStorage
  const [config, setConfig] = useState<PlaygroundConfig>(
    getInitialPlaygroundConfig
  )

  const [parameterEnabled, setParameterEnabled] = useState<ParameterEnabled>(
    getInitialParameterEnabled
  )

  const [messages, setMessages] = useState<Message[]>([])
  const [isLoadingMessages, setIsLoadingMessages] = useState(true)
  const messagesSaveTimerRef = useRef<number | null>(null)
  const latestMessagesRef = useRef<Message[]>(messages)
  const hasLoadedMessagesRef = useRef(false)
  const mediaObjectUrlsRef = useRef<Map<string, string>>(new Map())

  const [models, setModels] = useState<ModelOption[]>([])
  const [groups, setGroups] = useState<GroupOption[]>([])

  const persistMessages = useCallback((messagesToSave: Message[]) => {
    latestMessagesRef.current = messagesToSave

    if (!hasLoadedMessagesRef.current) {
      return
    }

    if (messagesSaveTimerRef.current !== null) {
      window.clearTimeout(messagesSaveTimerRef.current)
    }

    messagesSaveTimerRef.current = window.setTimeout(() => {
      messagesSaveTimerRef.current = null
      saveMessages(latestMessagesRef.current)
      void pruneStoredMedia(getStoredMediaKeys(latestMessagesRef.current))
    }, MESSAGE_SAVE_DEBOUNCE_MS)
  }, [])

  useEffect(() => {
    let cancelled = false

    window.setTimeout(() => {
      void (async () => {
        const loadedMessages = loadMessages() ?? []
        const hydrated = await hydrateStoredMedia(loadedMessages)
        if (cancelled) {
          for (const url of hydrated.objectUrls.values()) {
            URL.revokeObjectURL(url)
          }
          return
        }

        mediaObjectUrlsRef.current = hydrated.objectUrls
        latestMessagesRef.current = hydrated.messages
        hasLoadedMessagesRef.current = true
        setMessages(hydrated.messages)
        setIsLoadingMessages(false)
        void pruneStoredMedia(getStoredMediaKeys(hydrated.messages))
      })()
    }, 0)

    return () => {
      cancelled = true
    }
  }, [])

  useEffect(
    () => () => {
      if (messagesSaveTimerRef.current !== null) {
        window.clearTimeout(messagesSaveTimerRef.current)
        saveMessages(latestMessagesRef.current)
      }
      for (const url of mediaObjectUrlsRef.current.values()) {
        URL.revokeObjectURL(url)
      }
    },
    []
  )

  // Update config with automatic save
  const updateConfig = useCallback(
    <K extends keyof PlaygroundConfig>(key: K, value: PlaygroundConfig[K]) => {
      setConfig((prev) => {
        const updated = { ...prev, [key]: value }
        saveConfig(updated)
        return updated
      })
    },
    []
  )

  const updateModeSelection = useCallback(
    <K extends keyof PlaygroundModeSelection>(
      mode: PlaygroundMode,
      key: K,
      value: PlaygroundModeSelection[K]
    ) => {
      setConfig((prev) => {
        const updated = {
          ...prev,
          mode_selections: {
            ...prev.mode_selections,
            [mode]: { ...prev.mode_selections[mode], [key]: value },
          },
        }
        saveConfig(updated)
        return updated
      })
    },
    []
  )

  // Update parameter enabled with automatic save
  const updateParameterEnabled = useCallback(
    (key: keyof ParameterEnabled, value: boolean) => {
      setParameterEnabled((prev) => {
        const updated = { ...prev, [key]: value }
        saveParameterEnabled(updated)
        return updated
      })
    },
    []
  )

  // Update messages with automatic save
  const updateMessages = useCallback(
    (updater: MessageStateUpdater) => {
      setMessages((prev) => {
        const newMessages = applyMessageStateUpdate(prev, updater)
        const previousKeys = getStoredMediaKeys(prev)
        const nextKeys = getStoredMediaKeys(newMessages)
        const removedKeys = [...previousKeys].filter(
          (storageKey) => !nextKeys.has(storageKey)
        )
        for (const storageKey of removedKeys) {
          const url = mediaObjectUrlsRef.current.get(storageKey)
          if (url) URL.revokeObjectURL(url)
          mediaObjectUrlsRef.current.delete(storageKey)
        }
        void deleteStoredMedia(removedKeys)
        persistMessages(newMessages)
        return newMessages
      })
    },
    [persistMessages]
  )

  // Clear all messages
  const clearMessages = useCallback(() => {
    updateMessages([])
  }, [updateMessages])

  // Reset config to defaults
  const resetConfig = useCallback(() => {
    setConfig(DEFAULT_CONFIG)
    setParameterEnabled(DEFAULT_PARAMETER_ENABLED)
    saveConfig(DEFAULT_CONFIG)
    saveParameterEnabled(DEFAULT_PARAMETER_ENABLED)
  }, [])

  return {
    // State
    config,
    parameterEnabled,
    messages,
    isLoadingMessages,
    models,
    groups,

    // Setters
    setModels,
    setGroups,

    // Actions
    updateConfig,
    updateModeSelection,
    updateParameterEnabled,
    updateMessages,
    clearMessages,
    resetConfig,
  }
}

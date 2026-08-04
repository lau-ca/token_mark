import { useCallback, useEffect, useMemo, useState } from 'react'

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
import { PlaygroundChat } from './components/chat/playground-chat'
import { PlaygroundInput } from './components/input/playground-input'
import { PlaygroundMediaInput } from './components/media/playground-media-input'
import { PlaygroundContextBar } from './components/playground-context-bar'
import { STORAGE_KEYS } from './constants'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
  usePlaygroundVideoRecovery,
} from './hooks'
import { getModeModels } from './lib'
import type { PlaygroundMode } from './types'

export function Playground() {
  const [mode, setMode] = useState<PlaygroundMode>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEYS.MODE)
      return stored === 'image' || stored === 'video' ? stored : 'chat'
    } catch {
      return 'chat'
    }
  })
  const [drafts, setDrafts] = useState<Record<PlaygroundMode, string>>({
    chat: '',
    image: '',
    video: '',
  })
  const {
    config,
    parameterEnabled,
    messages,
    isLoadingMessages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    updateModeSelection,
    updateParameterEnabled,
  } = usePlaygroundState()

  usePlaygroundVideoRecovery({
    isLoadingMessages,
    messages,
    updateMessages,
  })

  const activeSelection = config.mode_selections[mode]
  const activeConfig = useMemo(
    () => ({ ...config, ...activeSelection }),
    [activeSelection, config]
  )
  const activeRequestContext = useMemo(
    () => ({ model: activeConfig.model, group: activeConfig.group }),
    [activeConfig.group, activeConfig.model]
  )
  const updateActiveConfig = useCallback(
    <K extends keyof typeof activeConfig>(
      key: K,
      value: (typeof activeConfig)[K]
    ) => {
      if (key === 'model' || key === 'group') {
        updateModeSelection(mode, key, String(value))
        return
      }
      updateConfig(key, value)
    },
    [mode, updateConfig, updateModeSelection]
  )
  const updateDraft = useCallback(
    (draftMode: PlaygroundMode, value: string) => {
      setDrafts((current) => ({ ...current, [draftMode]: value }))
    },
    []
  )

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config: activeConfig,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  const {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  } = usePlaygroundConversation({
    messages,
    updateMessages,
    sendChat,
    mode: 'chat',
    requestContext: activeRequestContext,
  })

  const handleClearMessages = () => {
    handleEditOpenChange(false)
    updateMessages((current) =>
      current.filter((message) => (message.mode ?? 'chat') !== mode)
    )
  }

  const { isLoadingModels, modelCatalog } = usePlaygroundOptions({
    currentGroup: activeConfig.group,
    currentMode: mode,
    currentModel: activeConfig.model,
    setGroups,
    setModels,
    updateConfig: updateActiveConfig,
  })

  const modeModels = useMemo(() => getModeModels(models, mode), [mode, models])
  const selectedModeModel = useMemo(
    () => modeModels.find((model) => model.value === activeConfig.model),
    [activeConfig.model, modeModels]
  )
  const modeMessages = messages.filter(
    (message) => (message.mode ?? 'chat') === mode
  )

  useEffect(() => {
    if (
      modeModels.length > 0 &&
      !modeModels.some((model) => model.value === activeConfig.model)
    ) {
      updateActiveConfig('model', modeModels[0].value)
    }
  }, [activeConfig.model, modeModels, updateActiveConfig])

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.MODE, mode)
    } catch {
      // Ignore unavailable storage and keep the in-memory selection.
    }
  }, [mode])

  return (
    <div className='relative flex size-full min-h-0 flex-col overflow-hidden'>
      <div className='mx-auto w-full max-w-5xl px-4 pt-4'>
        <PlaygroundContextBar
          groups={groups}
          isModelLoading={isLoadingModels}
          mode={mode}
          models={modeModels}
          modelCatalog={modelCatalog}
          onGroupChange={(value) => updateActiveConfig('group', value)}
          onModeChange={setMode}
          onModelChange={(value) => updateActiveConfig('model', value)}
          selectedGroup={activeConfig.group}
          selectedModel={activeConfig.model}
        />
      </div>
      {/* Full-width scroll container: scrolling works even over side whitespace */}
      <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
        <PlaygroundChat
          messages={modeMessages}
          isLoadingMessages={isLoadingMessages}
          onRegenerateMessage={
            mode === 'chat' ? handleRegenerateMessage : undefined
          }
          onEditMessage={mode === 'chat' ? handleEditMessage : undefined}
          onDeleteMessage={handleDeleteMessage}
          onSelectPrompt={mode === 'chat' ? handleSendMessage : undefined}
          isGenerating={isGenerating}
          editingKey={editingMessageKey}
          onCancelEdit={handleEditOpenChange}
          onSaveEdit={(newContent) => applyEdit(newContent, false)}
          onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
          mode={mode}
        />
      </div>

      {/* Input area: center content and constrain to the same container width */}
      <div className='mx-auto w-full max-w-5xl px-4 pb-4'>
        {mode === 'chat' ? (
          <PlaygroundInput
            config={activeConfig}
            disabled={isGenerating}
            hasModel={Boolean(activeConfig.model && modeModels.length > 0)}
            isGenerating={isGenerating}
            modelOption={selectedModeModel}
            onValueChange={(value) => updateDraft('chat', value)}
            onConfigChange={updateActiveConfig}
            onClearMessages={handleClearMessages}
            onParameterEnabledChange={updateParameterEnabled}
            onStop={stopGeneration}
            onSubmit={handleSendMessage}
            parameterEnabled={parameterEnabled}
            value={drafts.chat}
            hasMessages={modeMessages.length > 0}
          />
        ) : (
          <PlaygroundMediaInput
            key={mode}
            config={activeConfig}
            hasMessages={modeMessages.length > 0}
            mode={mode}
            models={modeModels}
            onClearMessages={handleClearMessages}
            onPromptChange={(value) => updateDraft(mode, value)}
            prompt={drafts[mode]}
            updateMessages={updateMessages}
          />
        )}
      </div>
    </div>
  )
}

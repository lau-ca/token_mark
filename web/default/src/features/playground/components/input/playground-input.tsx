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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  PromptInput,
  PromptInputFooter,
  PromptInputTextarea,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { useStatus } from '@/hooks/use-status'

import {
  buildPlaygroundChatCurl,
  buildChatCompletionPayload,
  getModeEndpoint,
  getSubmittableInputText,
  resolvePlaygroundIntegrationGuide,
  resolvePlaygroundApiBaseUrl,
} from '../../lib'
import type {
  ModelOption,
  ParameterEnabled,
  PlaygroundConfig,
} from '../../types'
import { PlaygroundInputControls } from './playground-input-controls'
import { PlaygroundInputTools } from './playground-input-tools'

interface PlaygroundInputProps {
  config: PlaygroundConfig
  onSubmit: (text: string) => void
  onValueChange: (value: string) => void
  onStop?: () => void
  disabled?: boolean
  isGenerating?: boolean
  hasModel: boolean
  hasMessages?: boolean
  modelOption?: ModelOption
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onClearMessages?: () => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
  value: string
}

export function PlaygroundInput({
  config,
  onSubmit,
  onValueChange,
  onStop,
  disabled,
  isGenerating,
  hasModel,
  hasMessages = false,
  modelOption,
  onConfigChange,
  onClearMessages,
  onParameterEnabledChange,
  parameterEnabled,
  value,
}: PlaygroundInputProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const apiBaseUrl = resolvePlaygroundApiBaseUrl(status)
  const endpointEntry = useMemo(
    () => (modelOption ? getModeEndpoint(modelOption, 'chat') : undefined),
    [modelOption]
  )
  const examplePrompt = value.trim() || '<YOUR_PROMPT>'
  const curl = useMemo(
    () =>
      buildPlaygroundChatCurl({
        apiBaseUrl,
        config,
        parameterEnabled,
        prompt: examplePrompt,
      }),
    [apiBaseUrl, config, examplePrompt, parameterEnabled]
  )
  const guide = useMemo(() => {
    if (!endpointEntry || !config.model || !apiBaseUrl) return null
    const definition = endpointEntry[1].playground?.integration
    if (!definition) return null
    const payload = buildChatCompletionPayload(
      [
        {
          key: 'integration-draft',
          from: 'user',
          versions: [{ id: 'integration-draft', content: examplePrompt }],
        },
      ],
      config,
      parameterEnabled
    )
    return resolvePlaygroundIntegrationGuide({
      apiBaseUrl,
      definition,
      group: config.group,
      model: config.model,
      parameters: Object.fromEntries(
        Object.entries(payload).filter(
          ([key]) => !['model', 'group', 'messages'].includes(key)
        )
      ) as Record<string, string | number | boolean>,
      prompt: examplePrompt,
      requestCurl: curl,
      requestJson: JSON.stringify(payload, null, 2),
    })
  }, [apiBaseUrl, config, curl, endpointEntry, examplePrompt, parameterEnabled])

  const handleSubmit = (message: PromptInputMessage) => {
    const submittableText = getSubmittableInputText(message, disabled)

    if (!submittableText) return
    onSubmit(submittableText)
    onValueChange('')
  }

  return (
    <div className='grid shrink-0 gap-4 px-1 md:pb-4'>
      <PromptInput
        className='relative'
        groupClassName='bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl overflow-hidden transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15 focus-within:shadow-[0_22px_70px_-34px_rgba(0,0,0,0.75)]'
        onSubmit={handleSubmit}
      >
        <PromptInputTextarea
          autoComplete='off'
          autoCorrect='off'
          autoCapitalize='off'
          spellCheck={false}
          className='min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base'
          disabled={disabled}
          onChange={(event) => onValueChange(event.target.value)}
          placeholder={t('Ask anything')}
          value={value}
        />

        <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5 backdrop-blur'>
          <PlaygroundInputControls
            disabled={disabled}
            hasModel={hasModel}
            isGenerating={isGenerating}
            onStop={onStop}
            text={value}
            tools={
              <PlaygroundInputTools
                apiBaseUrl={apiBaseUrl}
                config={config}
                disabled={disabled}
                guide={guide}
                hasMessages={hasMessages}
                onConfigChange={onConfigChange}
                onClearMessages={onClearMessages}
                onParameterEnabledChange={onParameterEnabledChange}
                parameterEnabled={parameterEnabled}
              />
            }
          />
        </PromptInputFooter>
      </PromptInput>
    </div>
  )
}

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

import { PaperclipIcon } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  PromptInput,
  PromptInputFooter,
  PromptInputTextarea,
  PromptInputTools,
} from '@/components/ai-elements/prompt-input'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { useStatus } from '@/hooks/use-status'
import { api } from '@/lib/api'

import { API_ENDPOINTS } from '../../constants'
import {
  completeAssistantMessage,
  completeAssistantMessageWithError,
  createLoadingAssistantMessage,
  createUserMessage,
  buildPlaygroundMediaCurl,
  buildPlaygroundMediaPayload,
  getEndpointCapabilityNames,
  getPlaygroundParameterValidationError,
  getModeEndpoint,
  persistGeneratedImages,
  parseRequestErrorDetails,
  resolvePlaygroundIntegrationGuide,
  resolvePlaygroundApiBaseUrl,
  updateAssistantMessageByKey,
  updateCurrentVersionContent,
} from '../../lib'
import {
  getImageResult,
  isCanceledMediaRequest,
  readFileAsDataURL,
  unwrapMediaResponse,
} from '../../lib/media/playground-media-utils'
import type {
  Message,
  ModelOption,
  PlaygroundMode,
  PlaygroundConfig,
  PlaygroundMedia,
} from '../../types'
import { PlaygroundApiIntegrationButton } from '../input/playground-api-integration-button'
import { PlaygroundClearHistoryButton } from '../input/playground-clear-history-button'
import { PlaygroundInputControls } from '../input/playground-input-controls'
import { PlaygroundMediaParameterPanel } from './playground-media-parameter-panel'
import type { PlaygroundParameterValues } from './playground-parameter-fields'

type Props = {
  config: PlaygroundConfig
  mode: Exclude<PlaygroundMode, 'chat'>
  models: ModelOption[]
  hasMessages: boolean
  onClearMessages: () => void
  onPromptChange: (value: string) => void
  prompt: string
  updateMessages: (updater: (messages: Message[]) => Message[]) => void
}

export function PlaygroundMediaInput(props: Props) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [values, setValues] = useState<PlaygroundParameterValues>({})
  const [referenceFile, setReferenceFile] = useState<File | null>(null)
  const [referenceUrl, setReferenceUrl] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [hasAttemptedSubmit, setHasAttemptedSubmit] = useState(false)
  const [parametersOpen, setParametersOpen] = useState(false)
  const requestControllerRef = useRef<AbortController | null>(null)

  const selectedModel =
    props.models.find((model) => model.value === props.config.model) ??
    props.models[0]
  const endpointEntry = useMemo(
    () =>
      selectedModel ? getModeEndpoint(selectedModel, props.mode) : undefined,
    [props.mode, selectedModel]
  )
  const parameters = useMemo(
    () => endpointEntry?.[1].playground?.parameters ?? [],
    [endpointEntry]
  )
  const capabilities = useMemo(
    () =>
      endpointEntry
        ? getEndpointCapabilityNames(endpointEntry[0], endpointEntry[1])
        : [],
    [endpointEntry]
  )
  const canGenerateImage = capabilities.includes('image.generate')
  const canEditImage = capabilities.includes('image.edit')
  const canGenerateVideoFromText = capabilities.includes('video.text_to_video')
  const canGenerateVideoFromImage = capabilities.includes(
    'video.image_to_video'
  )
  const apiBaseUrl = resolvePlaygroundApiBaseUrl(status)
  const examplePrompt = props.prompt.trim() || '<YOUR_PROMPT>'
  const curl = useMemo(
    () =>
      buildPlaygroundMediaCurl({
        apiBaseUrl,
        canEditImage,
        canGenerateVideoFromImage,
        endpointType: endpointEntry?.[0] ?? '',
        group: props.config.group,
        hasReferenceImage: Boolean(referenceFile),
        mode: props.mode,
        model: selectedModel?.value ?? '',
        parameters,
        prompt: examplePrompt,
        referenceUrl,
        values,
      }),
    [
      canEditImage,
      canGenerateVideoFromImage,
      endpointEntry,
      parameters,
      props.config.group,
      props.mode,
      examplePrompt,
      referenceFile,
      referenceUrl,
      selectedModel,
      apiBaseUrl,
      values,
    ]
  )
  const guide = useMemo(() => {
    if (!endpointEntry || !selectedModel || !apiBaseUrl) return null
    const definition = endpointEntry[1].playground?.integration
    if (!definition) return null
    const payload = buildPlaygroundMediaPayload({
      group: props.config.group,
      model: selectedModel.value,
      parameters,
      prompt: examplePrompt,
      values,
    })
    if (referenceUrl.trim() && canGenerateVideoFromImage) {
      payload.image = referenceUrl.trim()
    }
    return resolvePlaygroundIntegrationGuide({
      apiBaseUrl,
      definition,
      group: props.config.group,
      model: selectedModel.value,
      parameters: values,
      prompt: examplePrompt,
      referenceUrl: referenceUrl.trim() || undefined,
      requestCurl: curl,
      requestJson: JSON.stringify(payload, null, 2),
    })
  }, [
    apiBaseUrl,
    canGenerateVideoFromImage,
    curl,
    endpointEntry,
    parameters,
    props.config.group,
    examplePrompt,
    referenceUrl,
    selectedModel,
    values,
  ])

  useEffect(() => {
    const defaults: PlaygroundParameterValues = {}
    for (const parameter of parameters) {
      if (parameter.default !== undefined) {
        defaults[parameter.key] = parameter.default
      }
    }
    setValues(defaults)
    setParametersOpen(false)
  }, [parameters])

  useEffect(
    () => () => {
      requestControllerRef.current?.abort()
    },
    []
  )

  const updateAssistant = (
    messageKey: string,
    content: string,
    media?: PlaygroundMedia[]
  ) => {
    props.updateMessages((messages) =>
      messages.map((message) =>
        message.key === messageKey
          ? completeAssistantMessage({
              ...updateCurrentVersionContent(message, content),
              media,
            })
          : message
      )
    )
  }

  const updateImageAssistant = async (
    messageKey: string,
    response: Record<string, unknown>
  ) => {
    const result = getImageResult(response)
    const media = await persistGeneratedImages(result.media)
    if (
      media.some(
        (item) =>
          item.type === 'image' &&
          item.url?.startsWith('data:image/') &&
          !item.storageKey
      )
    ) {
      toast.warning(
        t('The image is available now but could not be saved in this browser')
      )
    }
    updateAssistant(
      messageKey,
      result.content === 'Image generation completed'
        ? t('Image generation completed')
        : result.content || t('Image not available'),
      media
    )
  }

  const updatePendingVideo = (messageKey: string, taskId: string) => {
    props.updateMessages((messages) =>
      updateAssistantMessageByKey(messages, messageKey, (message) => ({
        ...message,
        media: [{ type: 'video', taskId, progress: 0 }],
      }))
    )
  }

  const handleSubmit = async () => {
    if (!selectedModel || !endpointEntry || !props.prompt.trim()) return
    const hasInvalidParameter = parameters.some(
      (parameter) =>
        getPlaygroundParameterValidationError(
          parameter,
          values[parameter.key]
        ) !== null
    )
    if (hasInvalidParameter) {
      setHasAttemptedSubmit(true)
      setParametersOpen(true)
      toast.error(t('Check the highlighted parameters'))
      return
    }
    const requiresImageReference =
      (props.mode === 'image' && canEditImage && !canGenerateImage) ||
      (props.mode === 'video' &&
        canGenerateVideoFromImage &&
        !canGenerateVideoFromText)
    if (
      requiresImageReference &&
      ((props.mode === 'image' && !referenceFile) ||
        (props.mode === 'video' && !referenceUrl.trim()))
    ) {
      setHasAttemptedSubmit(true)
      setParametersOpen(true)
      toast.error(t('Reference image is required'))
      return
    }
    const submittedAt = Date.now()
    const userMessage = {
      ...createUserMessage(props.prompt.trim(), submittedAt),
      mode: props.mode,
    }
    const assistantMessage = {
      ...createLoadingAssistantMessage(submittedAt),
      mode: props.mode,
      requestContext: {
        model: selectedModel.value,
        group: props.config.group,
        parameters: Object.fromEntries(
          Object.entries(values).filter(
            ([, value]) => value !== undefined && value !== ''
          )
        ),
      },
    }
    props.updateMessages((messages) => [
      ...messages,
      userMessage,
      assistantMessage,
    ])

    try {
      setHasAttemptedSubmit(false)
      setIsSubmitting(true)
      setParametersOpen(false)
      const requestController = new AbortController()
      requestControllerRef.current = requestController
      const payload = buildPlaygroundMediaPayload({
        group: props.config.group,
        model: selectedModel.value,
        parameters,
        prompt: props.prompt,
        values,
      })

      if (props.mode === 'image') {
        if (endpointEntry[0] === 'gemini') {
          const imageConfig = payload.extra_body
          delete payload.prompt
          let content: unknown = props.prompt.trim()
          if (referenceFile && canEditImage) {
            const imageUrl = await readFileAsDataURL(referenceFile)
            content = [
              { type: 'text', text: props.prompt.trim() },
              { type: 'image_url', image_url: { url: imageUrl } },
            ]
          }
          payload.messages = [{ role: 'user', content }]
          payload.stream = false
          if (imageConfig) payload.extra_body = imageConfig
          const response = await api.post(
            API_ENDPOINTS.CHAT_COMPLETIONS,
            payload,
            {
              skipErrorHandler: true,
              signal: requestController.signal,
              headers: {
                'X-Playground-Capability': referenceFile
                  ? 'image.edit'
                  : 'image.generate',
              },
            } as Record<string, unknown>
          )
          await updateImageAssistant(
            assistantMessage.key,
            response.data as Record<string, unknown>
          )
        } else if (referenceFile && canEditImage) {
          const form = new FormData()
          form.append('model', selectedModel.value)
          form.append('group', props.config.group)
          form.append('prompt', props.prompt.trim())
          form.append('image', referenceFile)
          for (const [key, value] of Object.entries(values)) {
            form.append(key, String(value))
          }
          const response = await api.post(API_ENDPOINTS.IMAGE_EDITS, form, {
            skipErrorHandler: true,
            signal: requestController.signal,
          } as Record<string, unknown>)
          await updateImageAssistant(
            assistantMessage.key,
            response.data as Record<string, unknown>
          )
        } else {
          const response = await api.post(
            API_ENDPOINTS.IMAGE_GENERATIONS,
            payload,
            {
              skipErrorHandler: true,
              signal: requestController.signal,
            } as Record<string, unknown>
          )
          await updateImageAssistant(
            assistantMessage.key,
            response.data as Record<string, unknown>
          )
        }
      } else {
        if (referenceUrl.trim() && canGenerateVideoFromImage) {
          payload.image = referenceUrl.trim()
        }
        const response = await api.post(API_ENDPOINTS.VIDEOS, payload, {
          skipErrorHandler: true,
          signal: requestController.signal,
        } as Record<string, unknown>)
        const task = unwrapMediaResponse(
          response.data as Record<string, unknown>
        )
        const taskId = String(task.task_id ?? task.id ?? '')
        if (!taskId) throw new Error(t('Video task ID is missing'))
        updatePendingVideo(assistantMessage.key, taskId)
      }
      props.onPromptChange('')
    } catch (error: unknown) {
      if (isCanceledMediaRequest(error)) {
        props.updateMessages((messages) =>
          updateAssistantMessageByKey(
            messages,
            assistantMessage.key,
            (message) =>
              completeAssistantMessage({
                ...updateCurrentVersionContent(
                  message,
                  t('Generation stopped')
                ),
              })
          )
        )
        return
      }
      const errorDetails = parseRequestErrorDetails(error)
      const message = errorDetails.errorMessage || t('Operation failed')
      toast.error(message)
      props.updateMessages((messages) =>
        updateAssistantMessageByKey(messages, assistantMessage.key, (current) =>
          completeAssistantMessageWithError(
            current,
            message,
            errorDetails.errorCode,
            t('Request error occurred')
          )
        )
      )
    } finally {
      requestControllerRef.current = null
      setIsSubmitting(false)
    }
  }

  if (!selectedModel) {
    return (
      <Alert>
        <AlertTitle>{t('No compatible models')}</AlertTitle>
        <AlertDescription>
          {t('Choose another group from the model selector above.')}
        </AlertDescription>
      </Alert>
    )
  }

  const requiresImageReference =
    (props.mode === 'image' && canEditImage && !canGenerateImage) ||
    (props.mode === 'video' &&
      canGenerateVideoFromImage &&
      !canGenerateVideoFromText)
  const isReferenceMissing =
    requiresImageReference &&
    ((props.mode === 'image' && !referenceFile) ||
      (props.mode === 'video' && !referenceUrl.trim()))
  const hasParameterPanel =
    parameters.length > 0 ||
    (props.mode === 'image' && canEditImage) ||
    (props.mode === 'video' && canGenerateVideoFromImage)

  return (
    <PromptInput
      className='relative'
      groupClassName='bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl overflow-hidden transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15 focus-within:shadow-[0_22px_70px_-34px_rgba(0,0,0,0.75)]'
      onSubmit={() => handleSubmit()}
    >
      <PromptInputTextarea
        autoCapitalize='off'
        autoComplete='off'
        autoCorrect='off'
        className='min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base'
        disabled={isSubmitting}
        onChange={(event) => props.onPromptChange(event.target.value)}
        placeholder={
          props.mode === 'image'
            ? t('Describe the image you want to create')
            : t('Describe the video you want to create')
        }
        spellCheck={false}
        value={props.prompt}
      />

      <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5 backdrop-blur'>
        <PlaygroundInputControls
          disabled={isSubmitting}
          formNoValidate
          hasModel={Boolean(selectedModel)}
          isGenerating={isSubmitting}
          onStop={() => requestControllerRef.current?.abort()}
          submitLabel='Generate'
          text={props.prompt}
          tools={
            <>
              <PromptInputTools className='border-border/60 bg-background/70 rounded-lg border p-1 shadow-xs'>
                {hasParameterPanel && (
                  <PlaygroundMediaParameterPanel
                    canUseReference={
                      props.mode === 'image'
                        ? canEditImage
                        : canGenerateVideoFromImage
                    }
                    disabled={isSubmitting}
                    isReferenceMissing={isReferenceMissing}
                    mode={props.mode}
                    onOpenChange={setParametersOpen}
                    onReferenceFileChange={setReferenceFile}
                    onReferenceUrlChange={setReferenceUrl}
                    onValuesChange={setValues}
                    open={parametersOpen}
                    parameters={parameters}
                    referenceUrl={referenceUrl}
                    requiresReference={requiresImageReference}
                    showErrors={hasAttemptedSubmit}
                    values={values}
                  />
                )}

                <PlaygroundApiIntegrationButton
                  apiBaseUrl={apiBaseUrl}
                  disabled={isSubmitting}
                  guide={guide}
                  model={selectedModel.value}
                />

                <PlaygroundClearHistoryButton
                  descriptionKey='All messages in the current section saved in this browser will be removed. This cannot be undone.'
                  disabled={isSubmitting}
                  hasMessages={props.hasMessages}
                  labelKey='Clear history'
                  onClearMessages={props.onClearMessages}
                  successKey='History cleared'
                  titleKey='Clear current history?'
                />
              </PromptInputTools>

              {referenceFile && (
                <span className='text-muted-foreground hidden max-w-48 min-w-0 items-center gap-1 text-xs sm:flex'>
                  <PaperclipIcon className='size-3.5 shrink-0' />
                  <span className='truncate'>{referenceFile.name}</span>
                </span>
              )}
            </>
          }
        />
      </PromptInputFooter>
    </PromptInput>
  )
}

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
// Message types
export type MessageRole = 'user' | 'assistant' | 'system'

export type MessageStatus = 'loading' | 'streaming' | 'complete' | 'error'

export type PlaygroundMessageLayoutMode = 'alternating' | 'left'

export interface MessageVersion {
  id: string
  content: string
}

export interface PlaygroundMedia {
  type: 'image' | 'video'
  url?: string
  taskId?: string
  isTransient?: boolean
  storageKey?: string
  filename?: string
  progress?: number
}

export interface PlaygroundRequestContext {
  model: string
  group: string
  parameters?: Record<string, string | number | boolean>
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  createdAt?: number
  startedAt?: number
  completedAt?: number
  durationMs?: number
  sources?: { href: string; title: string }[]
  reasoning?: {
    content: string
    duration: number
    startedAt?: number
    completedAt?: number
    durationMs?: number
  }
  isReasoningStreaming?: boolean
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
  mode?: PlaygroundMode
  media?: PlaygroundMedia[]
  requestContext?: PlaygroundRequestContext
}

// API payload types
export interface ChatCompletionMessage {
  role: MessageRole
  content: string | ContentPart[]
}

export interface ContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: {
    url: string
  }
}

export interface ChatCompletionRequest {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  frequency_penalty?: number
  presence_penalty?: number
  seed?: number
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    delta: {
      role?: MessageRole
      content?: string
      reasoning_content?: string
    }
    finish_reason: string | null
  }>
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: {
      role: MessageRole
      content: string
      reasoning_content?: string
    }
    finish_reason: string
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

// Configuration types
export interface PlaygroundConfig {
  model: string
  group: string
  mode_selections: Record<PlaygroundMode, PlaygroundModeSelection>
  temperature: number
  top_p: number
  max_tokens: number
  frequency_penalty: number
  presence_penalty: number
  seed: number | null
  stream: boolean
}

export interface PlaygroundModeSelection {
  model: string
  group: string
}

export interface ParameterEnabled {
  temperature: boolean
  top_p: boolean
  max_tokens: boolean
  frequency_penalty: boolean
  presence_penalty: boolean
  seed: boolean
}

export interface PlaygroundParameterOption {
  key: string
  label?: string
  type: 'string' | 'number' | 'boolean' | 'enum'
  request_path?: string
  required?: boolean
  default?: string | number | boolean
  options?: Array<string | number | boolean>
  min?: number
  max?: number
}

export interface PlaygroundIntegrationInterface {
  key: string
  title: string
  description?: string
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  path: string
  request_description?: string
  curl_template: string
  response_example?: string
  notes?: string[]
}

export interface PlaygroundIntegrationDefinition {
  overview?: string
  documentation_url?: string
  interfaces: PlaygroundIntegrationInterface[]
  result_note?: string
  complete_example?: string
}

// Model and group options
export interface ModelOption {
  label: string
  value: string
  supportedEndpointTypes: string[]
  endpoints: Record<
    string,
    {
      path?: string
      method?: string
      playground?: {
        capabilities?: string[]
        parameters?: PlaygroundParameterOption[]
        integration?: PlaygroundIntegrationDefinition
      }
    }
  >
}

export type PlaygroundMode = 'chat' | 'image' | 'video'

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}

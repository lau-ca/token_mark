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

import type { ModelOption, PlaygroundMode } from '../../types'

const MODE_CAPABILITIES: Record<PlaygroundMode, string[]> = {
  chat: ['chat'],
  image: ['image.generate', 'image.edit'],
  video: ['video.text_to_video', 'video.image_to_video'],
}

export function getEndpointCapabilityNames(
  _endpointName: string,
  endpoint: ModelOption['endpoints'][string]
): string[] {
  return endpoint.playground?.capabilities ?? []
}

export function getModelCapabilityNames(model: ModelOption): string[] {
  const capabilities = new Set<string>()
  for (const [endpointName, endpoint] of Object.entries(model.endpoints)) {
    for (const capability of getEndpointCapabilityNames(
      endpointName,
      endpoint
    )) {
      capabilities.add(capability)
    }
  }
  return [...capabilities]
}

export function modelSupportsMode(
  model: ModelOption,
  mode: PlaygroundMode
): boolean {
  const capabilities = getModelCapabilityNames(model)
  return MODE_CAPABILITIES[mode].some((capability) =>
    capabilities.includes(capability)
  )
}

export function getModeModels(
  models: ModelOption[],
  mode: PlaygroundMode
): ModelOption[] {
  return models.filter((model) => modelSupportsMode(model, mode))
}

export function getModeEndpoint(model: ModelOption, mode: PlaygroundMode) {
  return Object.entries(model.endpoints).find(([endpointName, endpoint]) =>
    getEndpointCapabilityNames(endpointName, endpoint).some((capability) =>
      MODE_CAPABILITIES[mode].includes(capability)
    )
  )
}

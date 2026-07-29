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

import type { PlaygroundParameterOption } from '../../types'
import { setMediaRequestValue } from './playground-media-utils'

type BuildPlaygroundMediaPayloadOptions = {
  group: string
  model: string
  parameters: PlaygroundParameterOption[]
  prompt: string
  values: Record<string, string | number | boolean>
}

export function buildPlaygroundMediaPayload(
  options: BuildPlaygroundMediaPayloadOptions
): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    model: options.model,
    group: options.group,
    prompt: options.prompt.trim(),
  }

  for (const parameter of options.parameters) {
    const value = options.values[parameter.key]
    if (value === undefined || value === '') continue
    setMediaRequestValue(
      payload,
      parameter.request_path || parameter.key,
      value
    )
  }

  return payload
}

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

export type PlaygroundParameterValidationError = 'required' | 'range' | null

export function getPlaygroundParameterValidationError(
  parameter: PlaygroundParameterOption,
  value: string | number | boolean | undefined
): PlaygroundParameterValidationError {
  if (value === undefined || value === '') {
    return parameter.required ? 'required' : null
  }

  if (parameter.type !== 'number' || typeof value !== 'number') {
    return null
  }

  if (
    !Number.isFinite(value) ||
    (parameter.min !== undefined && value < parameter.min) ||
    (parameter.max !== undefined && value > parameter.max)
  ) {
    return 'range'
  }

  return null
}

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
import { Badge } from '@/components/ui/badge'

import type { PlaygroundRequestContext } from '../../types'

type PlaygroundMessageRequestContextProps = {
  context: PlaygroundRequestContext
}

const MAX_VISIBLE_PARAMETERS = 3

export function PlaygroundMessageRequestContext(
  props: PlaygroundMessageRequestContextProps
) {
  const parameters = Object.entries(props.context.parameters ?? {})
  const visibleParameters = parameters.slice(0, MAX_VISIBLE_PARAMETERS)
  const remainingCount = parameters.length - visibleParameters.length

  return (
    <div className='mb-2 flex flex-wrap items-center gap-1.5'>
      <Badge variant='secondary'>{props.context.model}</Badge>
      <Badge variant='outline'>{props.context.group}</Badge>
      {visibleParameters.map(([key, value]) => (
        <Badge key={key} variant='outline'>
          {key}={String(value)}
        </Badge>
      ))}
      {remainingCount > 0 && <Badge variant='outline'>+{remainingCount}</Badge>}
    </div>
  )
}

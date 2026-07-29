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

import { useTranslation } from 'react-i18next'

import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

import { PLAYGROUND_PARAMETER_PANEL_SCROLL_CLASS } from '../../lib'
import type { PlaygroundMode, PlaygroundParameterOption } from '../../types'
import { PlaygroundParameterPanelShell } from '../input/playground-parameter-panel-shell'
import {
  PlaygroundParameterFields,
  type PlaygroundParameterValues,
} from './playground-parameter-fields'

type Props = {
  canUseReference: boolean
  disabled?: boolean
  isReferenceMissing: boolean
  mode: Exclude<PlaygroundMode, 'chat'>
  onOpenChange: (open: boolean) => void
  onReferenceFileChange: (file: File | null) => void
  onReferenceUrlChange: (url: string) => void
  onValuesChange: (values: PlaygroundParameterValues) => void
  open: boolean
  parameters: PlaygroundParameterOption[]
  referenceUrl: string
  requiresReference: boolean
  showErrors: boolean
  values: PlaygroundParameterValues
}

export function PlaygroundMediaParameterPanel(props: Props) {
  const { t } = useTranslation()
  const activeCount = props.parameters.length + (props.canUseReference ? 1 : 0)

  const renderContent = (compact: boolean) => (
    <div
      className={cn(
        'grid gap-3',
        PLAYGROUND_PARAMETER_PANEL_SCROLL_CLASS,
        compact ? 'px-4 pb-4' : 'p-1'
      )}
    >
      <PlaygroundParameterFields
        className='sm:grid-cols-2 lg:grid-cols-2'
        onChange={props.onValuesChange}
        parameters={props.parameters}
        showErrors={props.showErrors}
        values={props.values}
      />

      {props.mode === 'image' && props.canUseReference && (
        <Field
          className='gap-1.5'
          data-invalid={props.showErrors && props.isReferenceMissing}
        >
          <FieldLabel htmlFor='playground-reference-image'>
            {t('Reference image')}
            {props.requiresReference && (
              <span className='text-muted-foreground text-xs font-normal'>
                {t('Required')}
              </span>
            )}
          </FieldLabel>
          <Input
            accept='image/*'
            aria-invalid={props.showErrors && props.isReferenceMissing}
            id='playground-reference-image'
            onChange={(event) =>
              props.onReferenceFileChange(event.target.files?.[0] ?? null)
            }
            type='file'
          />
          {props.showErrors && props.isReferenceMissing && (
            <FieldError className='text-xs'>
              {t('Reference image is required')}
            </FieldError>
          )}
        </Field>
      )}

      {props.mode === 'video' && props.canUseReference && (
        <Field
          className='gap-1.5'
          data-invalid={props.showErrors && props.isReferenceMissing}
        >
          <FieldLabel htmlFor='playground-reference-image-url'>
            {t('Reference image URL')}
            {props.requiresReference && (
              <span className='text-muted-foreground text-xs font-normal'>
                {t('Required')}
              </span>
            )}
          </FieldLabel>
          <Input
            aria-invalid={props.showErrors && props.isReferenceMissing}
            id='playground-reference-image-url'
            onChange={(event) => props.onReferenceUrlChange(event.target.value)}
            placeholder='https://example.com/image.png'
            type='url'
            value={props.referenceUrl}
          />
          {props.showErrors && props.isReferenceMissing && (
            <FieldError className='text-xs'>
              {t('Reference image is required')}
            </FieldError>
          )}
        </Field>
      )}
    </div>
  )

  return (
    <PlaygroundParameterPanelShell
      activeCount={activeCount}
      contentClassName='w-[36rem]'
      description={t(
        'These settings are sent with your image or video generation request.'
      )}
      disabled={props.disabled}
      onOpenChange={props.onOpenChange}
      open={props.open}
      renderContent={renderContent}
    />
  )
}

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
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react'
import type { UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import type {
  CompositeGroupFormInput,
  CompositeGroupOptions,
  CompositeRouteFormValue,
} from './types'

type RouteFieldName = 'generation_routes' | 'edit_routes'

type RouteTargetEditorProps = {
  form: UseFormReturn<CompositeGroupFormInput>
  name: RouteFieldName
  options: CompositeGroupOptions
  disabled: boolean
}

const emptyRoute: CompositeRouteFormValue = {
  client_key: '',
  physical_group: '',
  internal_model: '',
  retry_count: 0,
  retry_status_codes: '429,500-599',
}

export function RouteTargetEditor(props: RouteTargetEditorProps) {
  const { t } = useTranslation()
  const routes = props.form.watch(props.name)
  const routeErrors = props.form.formState.errors[props.name]
  const modelListId = `${props.name}-models`

  const getBillingLabel = (
    billingMode: CompositeGroupOptions['models'][number]['billingMode']
  ) => {
    if (billingMode === 'per-request') return t('Per request')
    if (billingMode === 'tiered') return t('Tiered pricing')
    return t('Per token')
  }

  const updateRoutes = (next: CompositeRouteFormValue[]) => {
    props.form.setValue(props.name, next, {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const moveRoute = (index: number, offset: -1 | 1) => {
    const target = index + offset
    if (target < 0 || target >= routes.length) return
    const next = [...routes]
    const current = next[index]
    next[index] = next[target]
    next[target] = current
    updateRoutes(next)
  }

  return (
    <FieldGroup className='gap-3'>
      {routes.map((route, index) => {
        const model = props.options.models.find(
          (item) => item.name === route.internal_model
        )
        const error = Array.isArray(routeErrors)
          ? routeErrors[index]
          : undefined
        return (
          <div
            key={route.client_key}
            className='border-border/70 bg-muted/20 rounded-xl border p-3'
          >
            <div className='mb-3 flex items-center justify-between gap-2'>
              <div className='flex items-center gap-2'>
                <Badge variant='outline'>
                  {t('Priority')} {index + 1}
                </Badge>
                {model && (
                  <Badge variant='secondary'>
                    {getBillingLabel(model.billingMode)}
                  </Badge>
                )}
              </div>
              <div className='flex items-center gap-1'>
                <Button
                  type='button'
                  size='icon-sm'
                  variant='ghost'
                  disabled={props.disabled || index === 0}
                  aria-label={t('Move route up')}
                  onClick={() => moveRoute(index, -1)}
                >
                  <ArrowUp aria-hidden='true' />
                </Button>
                <Button
                  type='button'
                  size='icon-sm'
                  variant='ghost'
                  disabled={props.disabled || index === routes.length - 1}
                  aria-label={t('Move route down')}
                  onClick={() => moveRoute(index, 1)}
                >
                  <ArrowDown aria-hidden='true' />
                </Button>
                <Button
                  type='button'
                  size='icon-sm'
                  variant='ghost'
                  disabled={props.disabled}
                  aria-label={t('Remove route')}
                  onClick={() =>
                    updateRoutes(
                      routes.filter((_, itemIndex) => itemIndex !== index)
                    )
                  }
                >
                  <Trash2 aria-hidden='true' />
                </Button>
              </div>
            </div>
            <FieldGroup className='grid gap-3 md:grid-cols-2'>
              <Field data-invalid={Boolean(error?.physical_group)}>
                <FieldLabel htmlFor={`${props.name}-${index}-group`}>
                  {t('Physical group')}
                </FieldLabel>
                <NativeSelect
                  id={`${props.name}-${index}-group`}
                  className='w-full'
                  aria-invalid={Boolean(error?.physical_group)}
                  disabled={props.disabled}
                  {...props.form.register(
                    `${props.name}.${index}.physical_group`
                  )}
                >
                  <NativeSelectOption value=''>
                    {t('Select a physical group')}
                  </NativeSelectOption>
                  {props.options.groups.map((group) => (
                    <NativeSelectOption key={group} value={group}>
                      {group}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
                <FieldError>
                  {error?.physical_group?.message
                    ? t(error.physical_group.message)
                    : undefined}
                </FieldError>
              </Field>
              <Field data-invalid={Boolean(error?.internal_model)}>
                <FieldLabel htmlFor={`${props.name}-${index}-model`}>
                  {t('Internal model')}
                </FieldLabel>
                <Input
                  id={`${props.name}-${index}-model`}
                  list={modelListId}
                  aria-invalid={Boolean(error?.internal_model)}
                  disabled={props.disabled}
                  {...props.form.register(
                    `${props.name}.${index}.internal_model`
                  )}
                />
                <FieldError>
                  {error?.internal_model?.message
                    ? t(error.internal_model.message)
                    : undefined}
                </FieldError>
              </Field>
              <Field data-invalid={Boolean(error?.retry_count)}>
                <FieldLabel htmlFor={`${props.name}-${index}-retry`}>
                  {t('Retry count')}
                </FieldLabel>
                <Input
                  id={`${props.name}-${index}-retry`}
                  type='number'
                  min={0}
                  max={10}
                  aria-invalid={Boolean(error?.retry_count)}
                  disabled={props.disabled}
                  {...props.form.register(
                    `${props.name}.${index}.retry_count`,
                    {
                      valueAsNumber: true,
                    }
                  )}
                />
                <FieldDescription>
                  {t('Retries exclude the initial request')}
                </FieldDescription>
                <FieldError>
                  {error?.retry_count?.message
                    ? t(error.retry_count.message)
                    : undefined}
                </FieldError>
              </Field>
              <Field data-invalid={Boolean(error?.retry_status_codes)}>
                <FieldLabel htmlFor={`${props.name}-${index}-statuses`}>
                  {t('Retry status codes')}
                </FieldLabel>
                <Input
                  id={`${props.name}-${index}-statuses`}
                  placeholder='429,500-599'
                  aria-invalid={Boolean(error?.retry_status_codes)}
                  disabled={props.disabled}
                  {...props.form.register(
                    `${props.name}.${index}.retry_status_codes`
                  )}
                />
                <FieldError>
                  {error?.retry_status_codes?.message
                    ? t(error.retry_status_codes.message)
                    : undefined}
                </FieldError>
              </Field>
            </FieldGroup>
          </div>
        )
      })}
      <datalist id={modelListId}>
        {props.options.models.map((model) => (
          <option key={model.name} value={model.name} />
        ))}
      </datalist>
      <Button
        type='button'
        variant='outline'
        disabled={props.disabled}
        onClick={() =>
          updateRoutes([
            ...routes,
            { ...emptyRoute, client_key: crypto.randomUUID() },
          ])
        }
      >
        <Plus data-icon='inline-start' aria-hidden='true' />
        {t('Add route')}
      </Button>
      <FieldError>
        {!Array.isArray(routeErrors) && routeErrors?.message
          ? t(routeErrors.message)
          : undefined}
      </FieldError>
    </FieldGroup>
  )
}

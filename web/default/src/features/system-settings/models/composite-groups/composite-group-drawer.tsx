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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  SideDrawerSection,
  SideDrawerSectionHeader,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { RouteTargetEditor } from './route-target-editor'
import { compositeGroupSchema, createCompositeGroupDefaults } from './schema'
import type {
  CompositeGroup,
  CompositeGroupFormInput,
  CompositeGroupOptions,
} from './types'

type CompositeGroupDrawerProps = {
  open: boolean
  group: CompositeGroup | null
  options: CompositeGroupOptions
  isSaving: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: CompositeGroupFormInput) => Promise<void>
}

export function CompositeGroupDrawer(props: CompositeGroupDrawerProps) {
  const { t } = useTranslation()
  const form = useForm<CompositeGroupFormInput>({
    resolver: zodResolver(compositeGroupSchema),
    defaultValues: createCompositeGroupDefaults(props.group),
  })

  useEffect(() => {
    if (props.open) {
      form.reset(createCompositeGroupDefaults(props.group))
    }
  }, [form, props.group, props.open])

  const generationEnabled = form.watch('generation_enabled')
  const editEnabled = form.watch('edit_enabled')

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {t(props.group ? 'Edit composite group' : 'Create composite group')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Expose one public image model and route it through ordered existing groups and models.'
            )}
          </SheetDescription>
        </SheetHeader>
        <form
          id='composite-group-form'
          className={sideDrawerFormClassName()}
          onSubmit={form.handleSubmit(props.onSubmit)}
        >
          <SideDrawerSection>
            <SideDrawerSectionHeader
              title={t('Public identity')}
              description={t(
                'Users select the group on their token and send the public request model.'
              )}
            />
            <FieldGroup className='grid gap-4 md:grid-cols-2'>
              <Field data-invalid={Boolean(form.formState.errors.name)}>
                <FieldLabel htmlFor='composite-name'>
                  {t('Composite group ID')}
                </FieldLabel>
                <Input
                  id='composite-name'
                  disabled={Boolean(props.group) || props.isSaving}
                  aria-invalid={Boolean(form.formState.errors.name)}
                  {...form.register('name')}
                />
                <FieldDescription>
                  {t('This is the group selected when creating a token.')}
                </FieldDescription>
                <FieldError>
                  {form.formState.errors.name?.message
                    ? t(form.formState.errors.name.message)
                    : undefined}
                </FieldError>
              </Field>
              <Field data-invalid={Boolean(form.formState.errors.public_model)}>
                <FieldLabel htmlFor='composite-public-model'>
                  {t('Public request model')}
                </FieldLabel>
                <Input
                  id='composite-public-model'
                  disabled={props.isSaving}
                  aria-invalid={Boolean(form.formState.errors.public_model)}
                  {...form.register('public_model')}
                />
                <FieldDescription>
                  {t(
                    'Clients send this model ID in generation and edit requests.'
                  )}
                </FieldDescription>
                <FieldError>
                  {form.formState.errors.public_model?.message
                    ? t(form.formState.errors.public_model.message)
                    : undefined}
                </FieldError>
              </Field>
              <Field>
                <FieldLabel htmlFor='composite-display-name'>
                  {t('Display name')}
                </FieldLabel>
                <Input
                  id='composite-display-name'
                  disabled={props.isSaving}
                  {...form.register('display_name')}
                />
              </Field>
              <Field className='md:col-span-2'>
                <FieldLabel htmlFor='composite-description'>
                  {t('Description')}
                </FieldLabel>
                <Textarea
                  id='composite-description'
                  disabled={props.isSaving}
                  {...form.register('description')}
                />
              </Field>
            </FieldGroup>
            <FieldGroup className='gap-3'>
              {(
                [
                  [
                    'status',
                    'Enabled',
                    'Allow this composite group to route requests.',
                  ],
                  [
                    'user_selectable',
                    'Selectable when creating tokens',
                    'Users can choose this group on the profile token form.',
                  ],
                  [
                    'pricing_visible',
                    'Show on pricing surfaces',
                    'Reserved for pricing-page presentation without changing physical pricing.',
                  ],
                ] as const
              ).map(([name, label, description]) => (
                <Controller
                  key={name}
                  control={form.control}
                  name={name}
                  render={({ field }) => (
                    <Field orientation='horizontal'>
                      <div className='flex-1'>
                        <FieldLabel htmlFor={`composite-${name}`}>
                          {t(label)}
                        </FieldLabel>
                        <FieldDescription>{t(description)}</FieldDescription>
                      </div>
                      <Switch
                        id={`composite-${name}`}
                        checked={field.value}
                        disabled={props.isSaving}
                        onCheckedChange={field.onChange}
                      />
                    </Field>
                  )}
                />
              ))}
            </FieldGroup>
          </SideDrawerSection>

          <SideDrawerSection>
            <Controller
              control={form.control}
              name='generation_enabled'
              render={({ field }) => (
                <Field orientation='horizontal'>
                  <div className='flex-1'>
                    <FieldLabel htmlFor='generation-enabled'>
                      {t('Generation routes')}
                    </FieldLabel>
                    <FieldDescription>
                      {t('Ordered targets for /v1/images/generations.')}
                    </FieldDescription>
                  </div>
                  <Switch
                    id='generation-enabled'
                    checked={field.value}
                    disabled={props.isSaving}
                    onCheckedChange={field.onChange}
                  />
                </Field>
              )}
            />
            <RouteTargetEditor
              form={form}
              name='generation_routes'
              options={props.options}
              disabled={props.isSaving || !generationEnabled}
            />
          </SideDrawerSection>

          <SideDrawerSection>
            <Controller
              control={form.control}
              name='edit_enabled'
              render={({ field }) => (
                <Field orientation='horizontal'>
                  <div className='flex-1'>
                    <FieldLabel htmlFor='edit-enabled'>
                      {t('Edit routes')}
                    </FieldLabel>
                    <FieldDescription>
                      {t('Ordered targets for /v1/images/edits and /v1/edits.')}
                    </FieldDescription>
                  </div>
                  <Switch
                    id='edit-enabled'
                    checked={field.value}
                    disabled={props.isSaving}
                    onCheckedChange={field.onChange}
                  />
                </Field>
              )}
            />
            <RouteTargetEditor
              form={form}
              name='edit_routes'
              options={props.options}
              disabled={props.isSaving || !editEnabled}
            />
          </SideDrawerSection>
        </form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button
            type='button'
            variant='outline'
            disabled={props.isSaving}
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='composite-group-form'
            disabled={props.isSaving}
          >
            {props.isSaving && <Spinner data-icon='inline-start' />}
            {t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

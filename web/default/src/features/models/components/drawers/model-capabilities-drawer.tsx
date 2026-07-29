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

import { Plus, Trash2 } from 'lucide-react'
import { nanoid } from 'nanoid'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'

import { createModel, updateModel } from '../../api'
import {
  getEndpointDefinition,
  parseModelEndpointDefinitions,
  serializeModelEndpointDefinitions,
  type PlaygroundCapability,
  type PlaygroundIntegrationDefinition,
  type PlaygroundParameterDefinition,
  type PlaygroundParameterType,
} from '../../lib/model-capabilities'
import type { ModelCapabilityCatalogItem } from '../../types'
import { ModelIntegrationTemplateEditor } from './model-integration-template-editor'

const CAPABILITIES: PlaygroundCapability[] = [
  'chat',
  'image.generate',
  'image.edit',
  'video.text_to_video',
  'video.image_to_video',
]

type EditablePlaygroundParameter = PlaygroundParameterDefinition & {
  editorId: string
}

type Props = {
  catalogItem: ModelCapabilityCatalogItem | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}

export function ModelCapabilitiesDrawer(props: Props) {
  const { t } = useTranslation()
  const [endpointName, setEndpointName] = useState('image-generation')
  const [capabilities, setCapabilities] = useState<PlaygroundCapability[]>([])
  const [parameters, setParameters] = useState<EditablePlaygroundParameter[]>(
    []
  )
  const [integration, setIntegration] = useState<
    PlaygroundIntegrationDefinition | undefined
  >()
  const [isSaving, setIsSaving] = useState(false)
  const endpointOptions = useMemo(() => {
    if (!props.catalogItem) return []
    const endpoints = parseModelEndpointDefinitions(
      props.catalogItem.metadata?.endpoints
    )
    return [
      ...new Set([
        ...Object.keys(endpoints),
        ...props.catalogItem.supported_endpoint_types.map(String),
      ]),
    ]
  }, [props.catalogItem])

  useEffect(() => {
    if (!props.catalogItem || !props.open) return
    const endpoints = parseModelEndpointDefinitions(
      props.catalogItem.metadata?.endpoints
    )
    const nextEndpoint =
      endpointOptions.find(
        (key) => getEndpointDefinition(endpoints, key).playground
      ) ??
      endpointOptions[0] ??
      ''
    const definition = getEndpointDefinition(endpoints, nextEndpoint)
    setEndpointName(nextEndpoint)
    setCapabilities(definition.playground?.capabilities ?? [])
    setParameters(
      (definition.playground?.parameters ?? []).map((parameter) => ({
        ...parameter,
        editorId: nanoid(),
      }))
    )
    setIntegration(definition.playground?.integration)
  }, [endpointOptions, props.catalogItem, props.open])

  const selectEndpoint = (name: string) => {
    if (!props.catalogItem) return
    const endpoints = parseModelEndpointDefinitions(
      props.catalogItem.metadata?.endpoints
    )
    const definition = getEndpointDefinition(endpoints, name)
    setEndpointName(name)
    setCapabilities(definition.playground?.capabilities ?? [])
    setParameters(
      (definition.playground?.parameters ?? []).map((parameter) => ({
        ...parameter,
        editorId: nanoid(),
      }))
    )
    setIntegration(definition.playground?.integration)
  }

  const toggleCapability = (capability: PlaygroundCapability) => {
    setCapabilities((current) =>
      current.includes(capability)
        ? current.filter((item) => item !== capability)
        : [...current, capability]
    )
  }

  const updateParameter = (
    index: number,
    patch: Partial<PlaygroundParameterDefinition>
  ) => {
    setParameters((current) =>
      current.map((parameter, parameterIndex) =>
        parameterIndex === index ? { ...parameter, ...patch } : parameter
      )
    )
  }

  const renderDefaultControl = (
    parameter: PlaygroundParameterDefinition,
    index: number
  ) => {
    if (parameter.type === 'boolean') {
      return (
        <Switch
          checked={parameter.default === true}
          onCheckedChange={(checked) =>
            updateParameter(index, { default: checked })
          }
        />
      )
    }
    return (
      <Input
        type={parameter.type === 'number' ? 'number' : 'text'}
        value={String(parameter.default ?? '')}
        onChange={(event) =>
          updateParameter(index, {
            default:
              parameter.type === 'number'
                ? Number(event.target.value)
                : event.target.value,
          })
        }
      />
    )
  }

  const handleSave = async () => {
    if (!props.catalogItem || !endpointName) return
    const parameterKeys = parameters.map((parameter) => parameter.key.trim())
    if (
      parameterKeys.some((key) => key === '') ||
      new Set(parameterKeys).size !== parameterKeys.length
    ) {
      toast.error(t('Parameter keys must be unique'))
      return
    }
    if (integration) {
      const interfaceKeys = integration.interfaces.map((item) =>
        item.key.trim()
      )
      const hasInvalidInterface = integration.interfaces.some(
        (item) =>
          !item.key.trim() ||
          !item.title.trim() ||
          !item.path.startsWith('/') ||
          !item.curl_template.trim()
      )
      if (
        integration.interfaces.length === 0 ||
        hasInvalidInterface ||
        new Set(interfaceKeys).size !== interfaceKeys.length
      ) {
        toast.error(t('Check the API integration template'))
        return
      }
    }
    const endpoints = parseModelEndpointDefinitions(
      props.catalogItem.metadata?.endpoints
    )
    const current = getEndpointDefinition(endpoints, endpointName)
    const savedParameters = parameters.map((parameter) => ({
      key: parameter.key,
      label: parameter.label,
      type: parameter.type,
      request_path: parameter.request_path,
      required: parameter.required,
      default: parameter.default,
      options: parameter.options,
      min: parameter.min,
      max: parameter.max,
    }))
    endpoints[endpointName] = {
      ...current,
      playground: {
        capabilities,
        parameters: savedParameters,
        integration,
      },
    }

    try {
      setIsSaving(true)
      const serializedEndpoints = serializeModelEndpointDefinitions(endpoints)
      const response = props.catalogItem.metadata
        ? await updateModel({
            ...props.catalogItem.metadata,
            endpoints: serializedEndpoints,
          })
        : await createModel({
            model_name: props.catalogItem.model_name,
            endpoints: serializedEndpoints,
            name_rule: 0,
            status: 1,
            sync_official: 1,
          })
      if (!response.success) {
        toast.error(response.message || t('Operation failed'))
        return
      }
      toast.success(t('Model capabilities updated'))
      props.onSaved()
      props.onOpenChange(false)
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      )
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Model capabilities')}</SheetTitle>
          <SheetDescription>{props.catalogItem?.model_name}</SheetDescription>
        </SheetHeader>

        <div className='flex-1 space-y-4 overflow-y-auto px-4 py-4'>
          <SideDrawerSection>
            <Label>{t('Endpoint Type')}</Label>
            <Select
              value={endpointName}
              onValueChange={(value) => value !== null && selectEndpoint(value)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {endpointOptions.map((name) => (
                    <SelectItem key={name} value={name}>
                      {name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </SideDrawerSection>

          <SideDrawerSection>
            <h3 className='text-sm font-semibold'>{t('User capabilities')}</h3>
            <div className='grid gap-3 sm:grid-cols-2'>
              {CAPABILITIES.map((capability) => (
                <Label
                  key={capability}
                  className='flex items-center gap-2 rounded-md border p-3'
                >
                  <Checkbox
                    checked={capabilities.includes(capability)}
                    onCheckedChange={() => toggleCapability(capability)}
                  />
                  {t(capability)}
                </Label>
              ))}
            </div>
          </SideDrawerSection>

          <SideDrawerSection>
            <div className='flex items-center justify-between gap-3'>
              <h3 className='text-sm font-semibold'>{t('User parameters')}</h3>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() =>
                  setParameters((current) => [
                    ...current,
                    {
                      editorId: nanoid(),
                      key: `parameter_${current.length + 1}`,
                      label: 'Custom parameter',
                      type: 'string',
                    },
                  ])
                }
              >
                <Plus className='size-4' />
                {t('Add parameter')}
              </Button>
            </div>
            <div className='space-y-3'>
              {parameters.map((parameter, index) => (
                <div key={parameter.editorId} className='rounded-md border p-3'>
                  <div className='grid gap-3 sm:grid-cols-2'>
                    <div className='space-y-1.5'>
                      <Label>{t('Parameter key')}</Label>
                      <Input
                        value={parameter.key}
                        onChange={(event) =>
                          updateParameter(index, { key: event.target.value })
                        }
                      />
                    </div>
                    <div className='space-y-1.5'>
                      <Label>{t('Display label')}</Label>
                      <Input
                        value={parameter.label ?? ''}
                        onChange={(event) =>
                          updateParameter(index, { label: event.target.value })
                        }
                      />
                    </div>
                    <div className='space-y-1.5'>
                      <Label>{t('Parameter type')}</Label>
                      <Select
                        value={parameter.type}
                        onValueChange={(value) =>
                          value !== null &&
                          updateParameter(index, {
                            type: value as PlaygroundParameterType,
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {['string', 'number', 'boolean', 'enum'].map(
                              (type) => (
                                <SelectItem key={type} value={type}>
                                  {type}
                                </SelectItem>
                              )
                            )}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className='space-y-1.5'>
                      <Label>{t('Default value')}</Label>
                      {renderDefaultControl(parameter, index)}
                    </div>
                    <div className='space-y-1.5 sm:col-span-2'>
                      <Label>{t('Request path')}</Label>
                      <Input
                        value={parameter.request_path ?? ''}
                        placeholder={parameter.key}
                        onChange={(event) =>
                          updateParameter(index, {
                            request_path: event.target.value,
                          })
                        }
                      />
                    </div>
                    {parameter.type === 'number' && (
                      <>
                        <div className='space-y-1.5'>
                          <Label>{t('Minimum')}</Label>
                          <Input
                            type='number'
                            value={parameter.min ?? ''}
                            onChange={(event) =>
                              updateParameter(index, {
                                min:
                                  event.target.value === ''
                                    ? undefined
                                    : Number(event.target.value),
                              })
                            }
                          />
                        </div>
                        <div className='space-y-1.5'>
                          <Label>{t('Maximum')}</Label>
                          <Input
                            type='number'
                            value={parameter.max ?? ''}
                            onChange={(event) =>
                              updateParameter(index, {
                                max:
                                  event.target.value === ''
                                    ? undefined
                                    : Number(event.target.value),
                              })
                            }
                          />
                        </div>
                      </>
                    )}
                    {parameter.type === 'enum' && (
                      <div className='space-y-1.5 sm:col-span-2'>
                        <Label>{t('Allowed values')}</Label>
                        <Input
                          value={(parameter.options ?? []).join(', ')}
                          onChange={(event) =>
                            updateParameter(index, {
                              options: event.target.value
                                .split(',')
                                .map((value) => value.trim())
                                .filter(Boolean),
                            })
                          }
                        />
                      </div>
                    )}
                    <Label className='flex items-center gap-2'>
                      <Switch
                        checked={parameter.required ?? false}
                        onCheckedChange={(checked) =>
                          updateParameter(index, { required: checked })
                        }
                      />
                      {t('Required')}
                    </Label>
                    <div className='flex justify-end'>
                      <Button
                        type='button'
                        size='icon'
                        variant='ghost'
                        aria-label={t('Delete parameter')}
                        onClick={() =>
                          setParameters((current) =>
                            current.filter(
                              (_, parameterIndex) => parameterIndex !== index
                            )
                          )
                        }
                      >
                        <Trash2 className='size-4' />
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </SideDrawerSection>

          <ModelIntegrationTemplateEditor
            capabilities={capabilities}
            endpointType={endpointName}
            model={props.catalogItem?.model_name ?? ''}
            onChange={setIntegration}
            parameters={parameters}
            value={integration}
          />
        </div>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={isSaving} onClick={handleSave}>
            {isSaving ? t('Saving...') : t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

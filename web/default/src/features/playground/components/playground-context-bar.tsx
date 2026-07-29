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

import { ModelGroupSelector } from '@/components/model-group-selector'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { GroupOption, ModelOption, PlaygroundMode } from '../types'

type PlaygroundContextBarProps = {
  groups: GroupOption[]
  isModelLoading: boolean
  mode: PlaygroundMode
  modelCatalog: Record<string, ModelOption[]>
  models: ModelOption[]
  onGroupChange: (value: string) => void
  onModeChange: (mode: PlaygroundMode) => void
  onModelChange: (value: string) => void
  selectedGroup: string
  selectedModel: string
}

export function PlaygroundContextBar(props: PlaygroundContextBarProps) {
  const { t } = useTranslation()

  return (
    <div className='border-border/70 bg-background/95 flex flex-col gap-3 rounded-xl border p-2.5 shadow-sm backdrop-blur lg:flex-row lg:items-center lg:justify-between'>
      <Tabs
        value={props.mode}
        onValueChange={(value) => props.onModeChange(value as PlaygroundMode)}
      >
        <TabsList className='grid w-full grid-cols-3 group-data-horizontal/tabs:h-9 lg:w-auto'>
          <TabsTrigger value='chat'>{t('Chat')}</TabsTrigger>
          <TabsTrigger value='image'>{t('Image creation')}</TabsTrigger>
          <TabsTrigger value='video'>{t('Video creation')}</TabsTrigger>
        </TabsList>
      </Tabs>

      <div className='flex min-w-0 flex-1 items-center gap-2 lg:max-w-[28rem] lg:justify-end'>
        <span className='text-muted-foreground hidden shrink-0 text-xs font-medium lg:inline'>
          {t('Model')}
        </span>
        <ModelGroupSelector
          className='h-9 w-full max-w-none lg:w-[24rem]'
          disabled={props.isModelLoading && props.groups.length === 0}
          groups={props.groups}
          modelGroups={props.modelCatalog}
          models={props.models}
          onGroupChange={props.onGroupChange}
          onModelChange={props.onModelChange}
          selectedGroup={props.selectedGroup}
          selectedModel={props.selectedModel}
        />
      </div>
    </div>
  )
}

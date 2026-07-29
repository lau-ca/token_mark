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

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Settings2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

import { getModelCapabilityCatalog } from '../api'
import { modelsQueryKeys } from '../lib'
import { getModelCapabilities } from '../lib/model-capabilities'
import type { ModelCapabilityCatalogItem } from '../types'
import { ModelCapabilitiesDrawer } from './drawers/model-capabilities-drawer'

export function ModelCapabilitiesTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState('')
  const [selectedModel, setSelectedModel] =
    useState<ModelCapabilityCatalogItem | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: modelsQueryKeys.capabilities(),
    queryFn: getModelCapabilityCatalog,
  })
  const models = (data?.data ?? []).filter((model) =>
    model.model_name.toLowerCase().includes(filter.trim().toLowerCase())
  )

  return (
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <Input
        value={filter}
        onChange={(event) => setFilter(event.target.value)}
        placeholder={t('Filter by model name...')}
        className='max-w-sm'
      />
      <div className='min-h-0 flex-1 space-y-3 overflow-y-auto pr-1'>
        {isLoading &&
          [0, 1, 2].map((item) => (
            <Skeleton key={item} className='h-20 w-full' />
          ))}
        {!isLoading && models.length === 0 && (
          <Empty>
            <EmptyHeader>
              <EmptyTitle>{t('No Models Found')}</EmptyTitle>
              <EmptyDescription>
                {t('No model capabilities to configure.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
        {models.map((model) => {
          const capabilities = getModelCapabilities(model.metadata?.endpoints)
          return (
            <Card key={model.model_name}>
              <CardContent className='flex items-center justify-between gap-4 py-4'>
                <div className='min-w-0 space-y-2'>
                  <div className='truncate font-medium'>{model.model_name}</div>
                  <div className='flex flex-wrap gap-1.5'>
                    {capabilities.length > 0 ? (
                      capabilities.map((capability) => (
                        <Badge key={capability} variant='secondary'>
                          {t(capability)}
                        </Badge>
                      ))
                    ) : (
                      <Badge variant='outline'>{t('Not configured')}</Badge>
                    )}
                  </div>
                </div>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => setSelectedModel(model)}
                >
                  <Settings2 className='size-4' />
                  {t('Configure')}
                </Button>
              </CardContent>
            </Card>
          )
        })}
      </div>
      <ModelCapabilitiesDrawer
        catalogItem={selectedModel}
        open={selectedModel !== null}
        onOpenChange={(open) => !open && setSelectedModel(null)}
        onSaved={() => {
          void queryClient.invalidateQueries({
            queryKey: modelsQueryKeys.capabilities(),
          })
        }}
      />
    </div>
  )
}

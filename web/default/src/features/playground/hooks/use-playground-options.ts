import { useQuery } from '@tanstack/react-query'
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
import { useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { getUserGroups, getUserModelCatalog, getUserModels } from '../api'
import {
  getGroupFallback,
  getModeModels,
  getModelFallback,
  getOptionLoadErrorMessage,
  shouldClearModelForGroup,
} from '../lib'
import type {
  GroupOption,
  ModelOption,
  PlaygroundConfig,
  PlaygroundMode,
} from '../types'

type UsePlaygroundOptionsParams = {
  currentGroup: string
  currentModel: string
  currentMode: PlaygroundMode
  setGroups: (groups: GroupOption[]) => void
  setModels: (models: ModelOption[]) => void
  updateConfig: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
}

export function usePlaygroundOptions({
  currentGroup,
  currentModel,
  currentMode,
  setGroups,
  setModels,
  updateConfig,
}: UsePlaygroundOptionsParams) {
  const { t } = useTranslation()

  const {
    data: modelsData,
    error: modelsError,
    isError: isModelsError,
    isLoading: isLoadingModels,
  } = useQuery({
    queryKey: ['playground-models', currentGroup],
    queryFn: () => getUserModels(currentGroup),
    enabled: currentGroup !== '',
  })

  const {
    data: groupsData,
    error: groupsError,
    isError: isGroupsError,
  } = useQuery({
    queryKey: ['playground-groups'],
    queryFn: getUserGroups,
  })

  const groupValues = useMemo(
    () => groupsData?.map((group) => group.value) ?? [],
    [groupsData]
  )
  const { data: modelCatalogData, isLoading: isLoadingModelCatalog } = useQuery(
    {
      queryKey: ['playground-model-catalog', groupValues],
      queryFn: () => getUserModelCatalog(groupsData ?? []),
      enabled: groupValues.length > 0,
      staleTime: 5 * 60 * 1000,
    }
  )
  const modeModelCatalog = useMemo(() => {
    if (!modelCatalogData) return {}

    return Object.fromEntries(
      Object.entries(modelCatalogData).map(([group, groupModels]) => [
        group,
        getModeModels(groupModels, currentMode),
      ])
    )
  }, [currentMode, modelCatalogData])

  useEffect(() => {
    if (!isModelsError) return

    toast.error(
      getOptionLoadErrorMessage(
        modelsError,
        t('Failed to load playground models')
      )
    )
  }, [isModelsError, modelsError, t])

  useEffect(() => {
    if (!isGroupsError) return

    toast.error(
      getOptionLoadErrorMessage(
        groupsError,
        t('Failed to load playground groups')
      )
    )
  }, [isGroupsError, groupsError, t])

  useEffect(() => {
    if (!modelsData) return

    setModels(modelsData)
    const compatibleModels = getModeModels(modelsData, currentMode)
    const fallback = getModelFallback(compatibleModels, currentModel)

    if (fallback) {
      updateConfig('model', fallback)
      return
    }

    if (shouldClearModelForGroup(compatibleModels, currentModel)) {
      updateConfig('model', '')
    }
  }, [modelsData, currentMode, currentModel, setModels, updateConfig])

  useEffect(() => {
    if (!groupsData) return

    const compatibleGroups = modelCatalogData
      ? groupsData.filter(
          (group) => (modeModelCatalog[group.value]?.length ?? 0) > 0
        )
      : groupsData
    setGroups(compatibleGroups)
    const fallback = getGroupFallback(compatibleGroups, currentGroup)

    if (fallback) {
      updateConfig('group', fallback)
    }
  }, [
    currentGroup,
    groupsData,
    modeModelCatalog,
    modelCatalogData,
    setGroups,
    updateConfig,
  ])

  return {
    isLoadingModels: isLoadingModels || isLoadingModelCatalog,
    modelCatalog: modeModelCatalog,
  }
}

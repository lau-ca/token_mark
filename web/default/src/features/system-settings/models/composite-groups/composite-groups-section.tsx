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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, RefreshCw, ShieldCheck, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'

import { SettingsSection } from '../../components/settings-section'
import { useUpdateOption } from '../../hooks/use-update-option'
import {
  createCompositeGroup,
  deleteCompositeGroup,
  getCompositeGroupOptions,
  listCompositeGroups,
  updateCompositeGroup,
  updateCompositeGroupStatus,
  validateCompositeGroup,
} from './api'
import { CompositeGroupDrawer } from './composite-group-drawer'
import { serializeCompositeGroup } from './schema'
import type {
  CompositeGroup,
  CompositeGroupFormInput,
  CompositeGroupOptions,
} from './types'

const queryKey = ['composite-groups'] as const
const optionsQueryKey = ['composite-group-options'] as const
const emptyOptions: CompositeGroupOptions = { groups: [], models: [] }

type CompositeGroupsSectionProps = {
  enabled: boolean
}

export function CompositeGroupsSection(props: CompositeGroupsSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingGroup, setEditingGroup] = useState<CompositeGroup | null>(null)
  const updateOption = useUpdateOption()

  const groupsQuery = useQuery({
    queryKey,
    queryFn: listCompositeGroups,
  })
  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
    queryFn: getCompositeGroupOptions,
  })

  const saveMutation = useMutation({
    mutationFn: async (values: CompositeGroupFormInput) => {
      const payload = serializeCompositeGroup(values)
      if (editingGroup) {
        return updateCompositeGroup(editingGroup.id, payload)
      }
      return createCompositeGroup(payload)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey })
      toast.success(t('Composite group saved'))
      setDrawerOpen(false)
      setEditingGroup(null)
    },
  })

  const statusMutation = useMutation({
    mutationFn: (group: CompositeGroup) =>
      updateCompositeGroupStatus(group.id, group.status === 1 ? 0 : 1),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey })
      toast.success(t('Composite group status updated'))
    },
  })

  const validateMutation = useMutation({
    mutationFn: validateCompositeGroup,
    onSuccess: () => toast.success(t('Composite group configuration is valid')),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteCompositeGroup,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey })
      toast.success(t('Composite group deleted'))
    },
  })

  const openCreate = () => {
    setEditingGroup(null)
    setDrawerOpen(true)
  }

  const openEdit = (group: CompositeGroup) => {
    setEditingGroup(group)
    setDrawerOpen(true)
  }

  const handleDelete = (group: CompositeGroup) => {
    if (
      !window.confirm(
        t('Delete composite group {{name}}?', { name: group.name })
      )
    ) {
      return
    }
    deleteMutation.mutate(group.id)
  }

  return (
    <SettingsSection title={t('Composite Groups')}>
      <Card>
        <CardHeader>
          <CardTitle>{t('Composite routing')}</CardTitle>
          <CardDescription>
            {t(
              'Enable composite group routing for API tokens. Keep this off until all relay workers run the same version.'
            )}
          </CardDescription>
          <CardAction>
            <Switch
              checked={props.enabled}
              disabled={updateOption.isPending}
              aria-label={t('Toggle composite routing')}
              onCheckedChange={(checked) =>
                updateOption.mutate({
                  key: 'CompositeGroupRoutingEnabled',
                  value: checked,
                })
              }
            />
          </CardAction>
        </CardHeader>
      </Card>

      <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
        <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
          {t(
            'Composite groups expose one administrator-defined image model while preserving each internal model and physical group billing rule.'
          )}
        </p>
        <div className='flex shrink-0 gap-2'>
          <Button
            variant='outline'
            disabled={groupsQuery.isFetching}
            onClick={() => groupsQuery.refetch()}
          >
            <RefreshCw data-icon='inline-start' aria-hidden='true' />
            {t('Refresh')}
          </Button>
          <Button onClick={openCreate}>
            <Plus data-icon='inline-start' aria-hidden='true' />
            {t('Create composite group')}
          </Button>
        </div>
      </div>

      {groupsQuery.isLoading && (
        <div className='grid gap-3 xl:grid-cols-2'>
          <Skeleton className='h-52 rounded-xl' />
          <Skeleton className='h-52 rounded-xl' />
        </div>
      )}
      {!groupsQuery.isLoading && Boolean(groupsQuery.data?.length) && (
        <div className='grid gap-3 xl:grid-cols-2'>
          {(groupsQuery.data ?? []).map((group) => {
            const generationRoutes = group.routes.filter(
              (route) =>
                route.operation === 'image_generation' && route.status === 1
            )
            const editRoutes = group.routes.filter(
              (route) => route.operation === 'image_edit' && route.status === 1
            )
            return (
              <Card key={group.id}>
                <CardHeader>
                  <CardTitle className='flex flex-wrap items-center gap-2'>
                    {group.display_name || group.name}
                    <Badge
                      variant={group.status === 1 ? 'secondary' : 'outline'}
                    >
                      {t(group.status === 1 ? 'Enabled' : 'Disabled')}
                    </Badge>
                  </CardTitle>
                  <CardDescription>
                    {group.name} · {group.public_model}
                  </CardDescription>
                  <CardAction>
                    <Switch
                      checked={group.status === 1}
                      disabled={statusMutation.isPending}
                      aria-label={t('Toggle composite group status')}
                      onCheckedChange={() => statusMutation.mutate(group)}
                    />
                  </CardAction>
                </CardHeader>
                <CardContent className='flex flex-col gap-4'>
                  {group.description && (
                    <p className='text-muted-foreground text-sm'>
                      {group.description}
                    </p>
                  )}
                  <div className='grid gap-3 sm:grid-cols-2'>
                    <RouteSummary
                      title={t('Generation routes')}
                      enabled={group.generation_enabled}
                      routes={generationRoutes}
                    />
                    <RouteSummary
                      title={t('Edit routes')}
                      enabled={group.edit_enabled}
                      routes={editRoutes}
                    />
                  </div>
                </CardContent>
                <CardFooter className='flex flex-wrap justify-end gap-2'>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={validateMutation.isPending}
                    onClick={() => validateMutation.mutate(group.id)}
                  >
                    <ShieldCheck data-icon='inline-start' aria-hidden='true' />
                    {t('Validate configuration')}
                  </Button>
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={() => openEdit(group)}
                  >
                    <Pencil data-icon='inline-start' aria-hidden='true' />
                    {t('Edit')}
                  </Button>
                  <Button
                    size='sm'
                    variant='destructive'
                    disabled={deleteMutation.isPending}
                    onClick={() => handleDelete(group)}
                  >
                    <Trash2 data-icon='inline-start' aria-hidden='true' />
                    {t('Delete')}
                  </Button>
                </CardFooter>
              </Card>
            )
          })}
        </div>
      )}
      {!groupsQuery.isLoading && !groupsQuery.data?.length && (
        <Empty className='border-border rounded-xl border'>
          <EmptyHeader>
            <EmptyTitle>{t('No composite groups')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'Create a composite group to configure ordered image fallback routes.'
              )}
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button onClick={openCreate}>{t('Create composite group')}</Button>
          </EmptyContent>
        </Empty>
      )}

      <CompositeGroupDrawer
        open={drawerOpen}
        group={editingGroup}
        options={optionsQuery.data ?? emptyOptions}
        isSaving={saveMutation.isPending}
        onOpenChange={(open) => {
          setDrawerOpen(open)
          if (!open) setEditingGroup(null)
        }}
        onSubmit={async (values) => {
          await saveMutation.mutateAsync(values)
        }}
      />
    </SettingsSection>
  )
}

function RouteSummary(props: {
  title: string
  enabled: boolean
  routes: CompositeGroup['routes']
}) {
  const { t } = useTranslation()
  return (
    <div className='border-border/70 rounded-lg border p-3'>
      <div className='mb-2 flex items-center justify-between gap-2'>
        <span className='text-sm font-medium'>{props.title}</span>
        <Badge variant='outline'>
          {t(props.enabled ? 'Enabled' : 'Disabled')}
        </Badge>
      </div>
      <div className='flex flex-col gap-1.5'>
        {props.routes.length ? (
          [...props.routes]
            .sort((a, b) => a.route_order - b.route_order)
            .map((route) => (
              <div
                key={route.id ?? `${route.operation}-${route.route_order}`}
                className='text-muted-foreground text-xs'
              >
                {route.route_order}. {route.physical_group} →{' '}
                {route.internal_model}
              </div>
            ))
        ) : (
          <span className='text-muted-foreground text-xs'>
            {t('No routes')}
          </span>
        )}
      </div>
    </div>
  )
}

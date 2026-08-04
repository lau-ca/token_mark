import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { User } from '@/features/users/types'

import {
  getAgentConfigurableGroups,
  getAgentProfile,
  updateAgentProfile,
} from '../api'

interface AgentSettingsDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  user?: User
  onSaved?: () => void
}

export function AgentSettingsDrawer(props: AgentSettingsDrawerProps) {
  const { t } = useTranslation()
  const [enabled, setEnabled] = useState(true)
  const [remark, setRemark] = useState('')
  const [marginRates, setMarginRates] = useState<Record<string, string>>({})
  const [retentionRates, setRetentionRates] = useState<Record<string, string>>(
    {}
  )
  const [isSubmitting, setIsSubmitting] = useState(false)

  const { data: groupsResponse } = useQuery({
    queryKey: ['agent-configurable-groups'],
    queryFn: getAgentConfigurableGroups,
    staleTime: 5 * 60 * 1000,
    enabled: props.open,
  })
  const groups = groupsResponse?.data ?? []

  useEffect(() => {
    if (!props.open || !props.user) return
    setEnabled(true)
    setRemark('')
    setMarginRates({})
    setRetentionRates({})
    if (!props.user.agent_enabled) return
    void getAgentProfile(props.user.id).then((response) => {
      if (!response.success || !response.data) return
      setEnabled(response.data.profile.enabled)
      setRemark(response.data.profile.remark ?? '')
      setMarginRates(
        Object.fromEntries(
          (response.data.current_version?.group_margins ?? []).map((item) => [
            item.group,
            String(item.gross_margin_rate * 100),
          ])
        )
      )
      setRetentionRates(
        Object.fromEntries(
          (response.data.current_version?.group_margins ?? []).map((item) => [
            item.group,
            String(item.platform_retention_rate * 100),
          ])
        )
      )
    })
  }, [props.open, props.user])

  const handleSave = async () => {
    if (!props.user) return
    const hasIncompleteGroup = groups.some((group) => {
      const hasGrossMargin = Boolean(marginRates[group]?.trim())
      const hasRetention = Boolean(retentionRates[group]?.trim())
      return hasGrossMargin !== hasRetention
    })
    if (hasIncompleteGroup) {
      toast.error(t('Complete both percentages for each configured group'))
      return
    }
    const groupMargins = groups.flatMap((group) => {
      if (!marginRates[group]?.trim() || !retentionRates[group]?.trim())
        return []
      const grossMargin = Number(marginRates[group])
      const retention = Number(retentionRates[group])
      if (!Number.isFinite(grossMargin) || grossMargin <= 0) return []
      if (!Number.isFinite(retention) || retention < 0) return []
      if (grossMargin > 100 || retention > 100) return []
      return [
        {
          group,
          gross_margin_rate: grossMargin / 100,
          platform_retention_rate: retention / 100,
        },
      ]
    })
    if (enabled && groupMargins.length === 0) {
      toast.error(t('Configure at least one group margin'))
      return
    }

    setIsSubmitting(true)
    try {
      const response = await updateAgentProfile(props.user.id, {
        enabled,
        remark,
        group_margins: groupMargins,
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to save agent settings'))
        return
      }
      toast.success(t('Agent settings saved'))
      props.onSaved?.()
      props.onOpenChange(false)
    } catch {
      toast.error(t('Failed to save agent settings'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[620px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Agent Settings')}</SheetTitle>
          <SheetDescription>
            {t('Configure profit sharing for {{username}}', {
              username: props.user?.username ?? '',
            })}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName()}>
          <SideDrawerSection>
            <div className='flex items-center justify-between gap-4'>
              <div>
                <Label>{t('Enable Agent')}</Label>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Only enabled agents can receive sales earnings.')}
                </p>
              </div>
              <Switch checked={enabled} onCheckedChange={setEnabled} />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='agent-remark'>{t('Admin Remark')}</Label>
              <Textarea
                id='agent-remark'
                value={remark}
                onChange={(event) => setRemark(event.target.value)}
                rows={3}
              />
            </div>
          </SideDrawerSection>

          <SideDrawerSection>
            <div>
              <h3 className='text-sm font-medium'>
                {t('Group Profit Sharing')}
              </h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Configure gross margin and platform retention for each user-selectable group.'
                )}
              </p>
            </div>
            <div className='space-y-4'>
              {groups.map((group) => (
                <div key={group} className='space-y-2 rounded-lg border p-3'>
                  <div className='font-medium'>{group}</div>
                  <div className='grid gap-3 sm:grid-cols-2'>
                    <div className='space-y-1.5'>
                      <Label htmlFor={`agent-margin-${group}`}>
                        {t('Group Gross Margin')}
                      </Label>
                      <div className='relative'>
                        <Input
                          id={`agent-margin-${group}`}
                          type='number'
                          min='0'
                          max='100'
                          step='0.01'
                          value={marginRates[group] ?? ''}
                          onChange={(event) =>
                            setMarginRates((current) => ({
                              ...current,
                              [group]: event.target.value,
                            }))
                          }
                          className='pr-8'
                        />
                        <span className='text-muted-foreground pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-xs'>
                          %
                        </span>
                      </div>
                    </div>
                    <div className='space-y-1.5'>
                      <Label htmlFor={`agent-retention-${group}`}>
                        {t('Platform Retention')}
                      </Label>
                      <div className='relative'>
                        <Input
                          id={`agent-retention-${group}`}
                          type='number'
                          min='0'
                          max='100'
                          step='0.01'
                          value={retentionRates[group] ?? ''}
                          onChange={(event) =>
                            setRetentionRates((current) => ({
                              ...current,
                              [group]: event.target.value,
                            }))
                          }
                          className='pr-8'
                        />
                        <span className='text-muted-foreground pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-xs'>
                          %
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </SideDrawerSection>
        </div>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button onClick={handleSave} disabled={isSubmitting}>
            {isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

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
import { getGroups } from '@/features/users/api'
import type { User } from '@/features/users/types'

import { getAgentProfile, updateAgentProfile } from '../api'

interface AgentSettingsDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  user?: User
  onSaved?: () => void
}

export function AgentSettingsDrawer(props: AgentSettingsDrawerProps) {
  const { t } = useTranslation()
  const [enabled, setEnabled] = useState(true)
  const [retentionPercent, setRetentionPercent] = useState('9')
  const [remark, setRemark] = useState('')
  const [marginRates, setMarginRates] = useState<Record<string, string>>({})
  const [isSubmitting, setIsSubmitting] = useState(false)

  const { data: groupsResponse } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
    staleTime: 5 * 60 * 1000,
    enabled: props.open,
  })
  const groups = groupsResponse?.data ?? []

  useEffect(() => {
    if (!props.open || !props.user) return
    setEnabled(true)
    setRetentionPercent('9')
    setRemark('')
    setMarginRates({})
    if (!props.user.agent_enabled) return
    void getAgentProfile(props.user.id).then((response) => {
      if (!response.success || !response.data) return
      setEnabled(response.data.profile.enabled)
      setRetentionPercent(
        String(response.data.profile.platform_retention_rate * 100)
      )
      setRemark(response.data.profile.remark ?? '')
      setMarginRates(
        Object.fromEntries(
          (response.data.current_version?.group_margins ?? []).map((item) => [
            item.group,
            String(item.gross_margin_rate * 100),
          ])
        )
      )
    })
  }, [props.open, props.user])

  const handleSave = async () => {
    if (!props.user) return
    const retention = Number(retentionPercent)
    if (!Number.isFinite(retention) || retention < 0 || retention > 100) {
      toast.error(t('Platform retention must be between 0% and 100%'))
      return
    }
    const groupMargins = groups.flatMap((group) => {
      const value = Number(marginRates[group])
      if (!Number.isFinite(value) || value <= 0) return []
      if (value > 100) return []
      return [{ group, gross_margin_rate: value / 100 }]
    })
    if (enabled && groupMargins.length === 0) {
      toast.error(t('Configure at least one group margin'))
      return
    }

    setIsSubmitting(true)
    try {
      const response = await updateAgentProfile(props.user.id, {
        enabled,
        platform_retention_rate: retention / 100,
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
              <Label htmlFor='agent-retention'>
                {t('Platform Retention Percentage')}
              </Label>
              <Input
                id='agent-retention'
                type='number'
                min='0'
                max='100'
                step='0.01'
                value={retentionPercent}
                onChange={(event) => setRetentionPercent(event.target.value)}
              />
              <p className='text-muted-foreground text-xs'>
                {t('The platform keeps this percentage of gross profit.')}
              </p>
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
              <h3 className='text-sm font-medium'>{t('Group Gross Margins')}</h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t('Leave a group empty to keep its agent earnings at zero.')}
              </p>
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              {groups.map((group) => (
                <div key={group} className='space-y-1.5'>
                  <Label htmlFor={`agent-margin-${group}`}>{group}</Label>
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

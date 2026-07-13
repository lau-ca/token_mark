import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BadgeDollarSign,
  CircleDollarSign,
  HandCoins,
  ReceiptText,
  Users,
  WalletCards,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { getDefaultTimeRange } from '@/features/usage-logs/lib/utils'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'
import type { User } from '@/features/users/types'

import {
  confirmAgentSettlement,
  getAgentProfiles,
  getAgentSettlements,
  getAgentStats,
  previewAgentSettlement,
} from './api'
import { AgentSettingsDrawer } from './components/agent-settings-drawer'

function latestSettlementCutoff() {
  return new Date(Date.now() - 60_000)
}

export function AgentUsers() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const authUser = useAuthStore((state) => state.auth.user)
  const isAdmin = Boolean(authUser && authUser.role >= ROLE.ADMIN)
  const [selectedAgentId, setSelectedAgentId] = useState<number>()
  const [timeRange, setTimeRange] = useState(getDefaultTimeRange)
  const [keyword, setKeyword] = useState('')
  const [group, setGroup] = useState('')
  const [page, setPage] = useState(1)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [settlementOpen, setSettlementOpen] = useState(false)
  const [paymentReference, setPaymentReference] = useState('')
  const [cutoff, setCutoff] = useState<Date>(latestSettlementCutoff)

  const { data: profilesResponse } = useQuery({
    queryKey: ['agent-profiles'],
    queryFn: () => getAgentProfiles(false),
    enabled: isAdmin,
    staleTime: 60_000,
  })
  const profiles = profilesResponse?.data ?? []
  const activeAgentId = isAdmin
    ? (selectedAgentId ?? profiles[0]?.user_id)
    : undefined
  const activeProfile = profiles.find(
    (profile) => profile.user_id === activeAgentId
  )

  const statsParams = useMemo(
    () => ({
      start_timestamp: Math.floor(timeRange.start.getTime() / 1000),
      end_timestamp: Math.floor(timeRange.end.getTime() / 1000),
      p: page,
      page_size: 20,
      keyword,
      group,
    }),
    [group, keyword, page, timeRange.end, timeRange.start]
  )
  const summaryParams = useMemo(
    () => ({
      start_timestamp: 1,
      p: 1,
      page_size: 1,
      keyword: '',
      group: '',
    }),
    []
  )

  const { data: statsResponse, isLoading } = useQuery({
    queryKey: ['agent-stats', activeAgentId ?? 'self', statsParams],
    queryFn: () => getAgentStats(statsParams, activeAgentId),
    enabled: !isAdmin || Boolean(activeAgentId),
  })
  const stats = statsResponse?.data
  const { data: summaryResponse } = useQuery({
    queryKey: ['agent-summary', activeAgentId ?? 'self'],
    queryFn: () =>
      getAgentStats(
        {
          ...summaryParams,
          end_timestamp: Math.floor(latestSettlementCutoff().getTime() / 1000),
        },
        activeAgentId
      ),
    enabled: !isAdmin || Boolean(activeAgentId),
    refetchInterval: 60_000,
  })
  const summary = summaryResponse?.data?.summary
  const periodSummary = stats?.summary

  const { data: settlementsResponse } = useQuery({
    queryKey: ['agent-settlements', activeAgentId ?? 'self'],
    queryFn: () => getAgentSettlements(activeAgentId),
    enabled: !isAdmin || Boolean(activeAgentId),
  })
  const settlements = settlementsResponse?.data ?? []

  const settlementMutation = useMutation({
    mutationFn: async () => {
      if (!activeAgentId) throw new Error('missing agent')
      const cutoffTimestamp = Math.floor(cutoff.getTime() / 1000)
      const preview = await previewAgentSettlement(
        activeAgentId,
        cutoffTimestamp
      )
      if (!preview.success) throw new Error(preview.message)
      return confirmAgentSettlement(
        activeAgentId,
        cutoffTimestamp,
        paymentReference
      )
    },
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to confirm settlement'))
        return
      }
      toast.success(t('Settlement confirmed'))
      setSettlementOpen(false)
      setPaymentReference('')
      await queryClient.invalidateQueries({ queryKey: ['agent-stats'] })
      await queryClient.invalidateQueries({ queryKey: ['agent-summary'] })
      await queryClient.invalidateQueries({ queryKey: ['agent-settlements'] })
    },
    onError: () => toast.error(t('Failed to confirm settlement')),
  })

  const summaryCards = [
    {
      title: t('Customers'),
      value: periodSummary?.customer_count ?? 0,
      description: t('Customers assigned to this agent'),
      icon: Users,
      tone: 'accent-1' as const,
    },
    {
      title: t('Consumption Amount'),
      value: formatQuota(periodSummary?.consumption_quota ?? 0),
      description: t('Consumption in the selected period'),
      icon: WalletCards,
      tone: 'accent-2' as const,
    },
    {
      title: t('Agent Earnings'),
      value: formatQuota(summary?.agent_earnings_quota ?? 0),
      description: t('Cumulative calculated earnings'),
      icon: CircleDollarSign,
      tone: 'accent-3' as const,
    },
    {
      title: t('Settled Amount'),
      value: formatQuota(summary?.settled_quota ?? 0),
      description: t('Confirmed offline payments'),
      icon: ReceiptText,
      tone: 'accent-1' as const,
    },
    {
      title: t('Pending Settlement'),
      value: formatQuota(summary?.pending_quota ?? 0),
      description: t('Earnings awaiting offline payment'),
      icon: HandCoins,
      tone: 'accent-2' as const,
    },
  ]
  if (isAdmin) {
    summaryCards.push({
      title: t('Platform Retained'),
      value: formatQuota(summary?.platform_retained_quota ?? 0),
      description: t('Gross profit retained by the platform'),
      icon: BadgeDollarSign,
      tone: 'accent-3' as const,
    })
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Agent Users')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {isAdmin && activeAgentId && (
            <>
              <Button variant='outline' onClick={() => setSettingsOpen(true)}>
                {t('Agent Settings')}
              </Button>
              <Button onClick={() => setSettlementOpen(true)}>
                {t('Confirm Settlement')}
              </Button>
            </>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4 px-3 pb-6 sm:px-4'>
            <Card size='sm'>
              <CardContent className='flex flex-wrap items-end gap-3'>
                {isAdmin && (
                  <div className='min-w-56 space-y-1.5'>
                    <Label>{t('Agent')}</Label>
                    <Select
                      items={profiles.map((profile) => ({
                        value: String(profile.user_id),
                        label: `${profile.username} (#${profile.user_id})`,
                      }))}
                      value={activeAgentId ? String(activeAgentId) : null}
                      onValueChange={(value) => {
                        if (value !== null) {
                          setSelectedAgentId(Number(value))
                          setPage(1)
                        }
                      }}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue placeholder={t('Select an agent')} />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {profiles.map((profile) => (
                            <SelectItem
                              key={profile.user_id}
                              value={String(profile.user_id)}
                            >
                              {profile.username} (#{profile.user_id})
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>
                )}
                <div className='min-w-72 flex-1'>
                  <CompactDateTimeRangePicker
                    start={timeRange.start}
                    end={timeRange.end}
                    onChange={({ start, end }) => {
                      if (!start || !end) return
                      setTimeRange({ start, end })
                      setPage(1)
                    }}
                  />
                </div>
                <div className='min-w-44 flex-1 space-y-1.5'>
                  <Label htmlFor='agent-customer-search'>{t('Customer')}</Label>
                  <Input
                    id='agent-customer-search'
                    value={keyword}
                    onChange={(event) => {
                      setKeyword(event.target.value)
                      setPage(1)
                    }}
                    placeholder={t('Search by user ID or username')}
                  />
                </div>
                <div className='min-w-36 space-y-1.5'>
                  <Label htmlFor='agent-group-filter'>{t('Group')}</Label>
                  <Input
                    id='agent-group-filter'
                    value={group}
                    onChange={(event) => {
                      setGroup(event.target.value)
                      setPage(1)
                    }}
                    placeholder={t('All groups')}
                  />
                </div>
              </CardContent>
            </Card>

            <div className='bg-card grid grid-cols-2 gap-px overflow-hidden rounded-xl border sm:grid-cols-3 xl:grid-cols-6'>
              {summaryCards.map((card) => (
                <div key={card.title} className='bg-background p-3'>
                  <StatCard {...card} loading={isLoading} compactMobile />
                </div>
              ))}
            </div>

            <Tabs defaultValue='earnings'>
              <TabsList>
                <TabsTrigger value='earnings'>{t('Earnings Detail')}</TabsTrigger>
                <TabsTrigger value='settlements'>
                  {t('Settlement History')}
                </TabsTrigger>
              </TabsList>
              <TabsContent value='earnings' className='mt-3'>
                <Card className='py-0'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Customer')}</TableHead>
                        <TableHead>{t('Group')}</TableHead>
                        <TableHead>{t('Consumption Amount')}</TableHead>
                        <TableHead>{t('Gross Margin')}</TableHead>
                        <TableHead>{t('Gross Profit')}</TableHead>
                        {isAdmin && <TableHead>{t('Platform Retained')}</TableHead>}
                        <TableHead>{t('Agent Earnings')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(stats?.details ?? []).map((row) => (
                        <TableRow key={`${row.customer_user_id}-${row.group}`}>
                          <TableCell>
                            <div className='font-medium'>{row.username}</div>
                            <div className='text-muted-foreground text-xs'>#{row.customer_user_id}</div>
                          </TableCell>
                          <TableCell>{row.group}</TableCell>
                          <TableCell>{formatQuota(row.consumption_quota)}</TableCell>
                          <TableCell>
                            {row.configured && row.gross_margin_rate != null
                              ? `${(row.gross_margin_rate * 100).toFixed(2)}%`
                              : t('Awaiting Configuration')}
                          </TableCell>
                          <TableCell>{formatQuota(row.gross_profit_quota)}</TableCell>
                          {isAdmin && <TableCell>{formatQuota(row.platform_retained_quota ?? 0)}</TableCell>}
                          <TableCell className='font-medium'>{formatQuota(row.agent_earnings_quota)}</TableCell>
                        </TableRow>
                      ))}
                      {!isLoading && (stats?.details.length ?? 0) === 0 && (
                        <TableRow>
                          <TableCell colSpan={isAdmin ? 7 : 6} className='text-muted-foreground h-28 text-center'>
                            {t('No agent earnings data')}
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                  <div className='flex items-center justify-between border-t p-3'>
                    <span className='text-muted-foreground text-xs'>
                      {t('{{count}} records', { count: stats?.total ?? 0 })}
                    </span>
                    <div className='flex gap-2'>
                      <Button variant='outline' size='sm' disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>
                        {t('Previous')}
                      </Button>
                      <Button variant='outline' size='sm' disabled={page * 20 >= (stats?.total ?? 0)} onClick={() => setPage((value) => value + 1)}>
                        {t('Next')}
                      </Button>
                    </div>
                  </div>
                </Card>
              </TabsContent>
              <TabsContent value='settlements' className='mt-3'>
                <Card className='py-0'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Settlement Period')}</TableHead>
                        <TableHead>{t('Agent Earnings')}</TableHead>
                        <TableHead>{t('Payment Reference')}</TableHead>
                        <TableHead>{t('Confirmed At')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {settlements.map((row) => (
                        <TableRow key={row.id}>
                          <TableCell>{formatTimestamp(row.period_start)} — {formatTimestamp(row.period_end)}</TableCell>
                          <TableCell className='font-medium'>{formatQuota(row.agent_earnings_quota)}</TableCell>
                          <TableCell>{row.payment_reference || '-'}</TableCell>
                          <TableCell>{formatTimestamp(row.confirmed_at)}</TableCell>
                        </TableRow>
                      ))}
                      {settlements.length === 0 && (
                        <TableRow>
                          <TableCell colSpan={4} className='text-muted-foreground h-28 text-center'>
                            {t('No settlement records')}
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                </Card>
              </TabsContent>
            </Tabs>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <AgentSettingsDrawer
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        user={
          activeProfile
            ? ({ id: activeProfile.user_id, username: activeProfile.username, display_name: activeProfile.display_name ?? '', role: activeProfile.role, status: 1, quota: 0, used_quota: 0, request_count: 0, group: '', agent_enabled: activeProfile.enabled } as User)
            : undefined
        }
        onSaved={() => {
          void queryClient.invalidateQueries({ queryKey: ['agent-profiles'] })
          void queryClient.invalidateQueries({ queryKey: ['agent-stats'] })
          void queryClient.invalidateQueries({ queryKey: ['agent-summary'] })
        }}
      />

      <Dialog open={settlementOpen} onOpenChange={setSettlementOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Confirm Settlement')}</DialogTitle>
            <DialogDescription>
              {t('Record an offline payment through the selected cutoff time.')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-1.5'>
              <Label>{t('Settlement Cutoff')}</Label>
              <DateTimePicker value={cutoff} onChange={(date) => date && setCutoff(date)} />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='agent-payment-reference'>{t('Payment Reference')}</Label>
              <Textarea id='agent-payment-reference' value={paymentReference} onChange={(event) => setPaymentReference(event.target.value)} rows={3} />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setSettlementOpen(false)}>{t('Cancel')}</Button>
            <Button disabled={settlementMutation.isPending} onClick={() => settlementMutation.mutate()}>
              {settlementMutation.isPending ? t('Confirming...') : t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

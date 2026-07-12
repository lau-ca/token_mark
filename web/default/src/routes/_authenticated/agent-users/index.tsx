import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { AgentUsers } from '@/features/agents'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

const agentUsersSearchSchema = z.object({
  agentId: z.number().optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/agent-users/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (
      !auth.user ||
      (auth.user.role < ROLE.ADMIN && auth.user.agent_enabled !== true)
    ) {
      throw redirect({ to: '/403' })
    }
  },
  validateSearch: agentUsersSearchSchema,
  component: AgentUsers,
})

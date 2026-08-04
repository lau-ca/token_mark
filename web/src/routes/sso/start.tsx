import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
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
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

type SSOIssueResponse = {
  success?: boolean
  data?: {
    redirect_url?: string
  }
}

function SSOStart() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = useSearch({ from: '/sso/start' }) as {
    client?: string
    return_to?: string
  }

  useEffect(() => {
    ;(async () => {
      const client = search.client || 'image'
      const returnTo = search.return_to || ''
      if (!returnTo) {
        navigate({ to: '/sign-in', replace: true })
        return
      }

      try {
        const res = await api.post<SSOIssueResponse>(
          '/api/sso/issue',
          {
            client,
            return_to: returnTo,
          },
          { skipErrorHandler: true } as Record<string, unknown>
        )
        const redirectUrl = res.data?.data?.redirect_url
        if (res.data?.success && redirectUrl) {
          window.location.replace(redirectUrl)
          return
        }
      } catch {
        useAuthStore.getState().auth.reset()
      }

      const redirect = `/sso/start?client=${encodeURIComponent(
        client
      )}&return_to=${encodeURIComponent(returnTo)}`
      navigate({ to: '/sign-in', search: { redirect }, replace: true })
    })()
  }, [navigate, search.client, search.return_to])

  return (
    <main className='bg-background text-foreground flex min-h-screen items-center justify-center'>
      <Loader2 className='text-muted-foreground h-5 w-5 animate-spin' />
      <span className='sr-only'>{t('Signing in')}</span>
    </main>
  )
}

export const Route = createFileRoute('/sso/start')({
  component: SSOStart,
})

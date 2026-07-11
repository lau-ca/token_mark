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
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type { SystemStatus } from '@/features/auth/types'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { useStatus } from '@/hooks/use-status'

function resolveApiBaseUrl(status: SystemStatus | null): string {
  const candidates = [
    status?.public_api_base_url,
    status?.data?.public_api_base_url,
    status?.server_address,
    status?.data?.server_address,
  ]

  for (const candidate of candidates) {
    if (typeof candidate === 'string' && candidate.trim()) {
      return candidate.trim().replace(/\/+$/, '')
    }
  }

  if (typeof window !== 'undefined') {
    return window.location.origin
  }

  return ''
}

export function ApiEndpointDisplay() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const apiBaseUrl = resolveApiBaseUrl(status)
  const isCopied = copiedText === apiBaseUrl
  const tooltip = isCopied ? t('Copied!') : t('Copy to clipboard')

  const display = (
    <Button
      type='button'
      variant='outline'
      onClick={() => void copyToClipboard(apiBaseUrl)}
      disabled={!apiBaseUrl}
      aria-label={`${t('Copy to clipboard')}: ${apiBaseUrl}`}
      className='h-auto min-h-9 min-w-0 flex-1 shrink justify-start gap-2 px-3 py-2 text-left'
    >
      <span className='shrink-0 font-semibold'>{t('API Endpoint')}</span>
      <Badge variant='secondary'>{t('Default')}</Badge>
      <span aria-hidden='true' className='bg-border h-4 w-px shrink-0' />
      <span className='text-muted-foreground min-w-0 flex-1 truncate font-mono font-normal'>
        {apiBaseUrl}
      </span>
      {isCopied ? <Check className='text-success' /> : <Copy />}
    </Button>
  )

  return (
    <Tooltip>
      <TooltipTrigger render={display} />
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  )
}

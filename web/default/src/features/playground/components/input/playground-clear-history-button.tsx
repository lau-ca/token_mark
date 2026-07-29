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

import { Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { PromptInputButton } from '@/components/ai-elements/prompt-input'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

type Props = {
  descriptionKey?: string
  disabled?: boolean
  hasMessages: boolean
  labelKey?: string
  onClearMessages?: () => void
  successKey?: string
  titleKey?: string
}

export function PlaygroundClearHistoryButton(props: Props) {
  const { t } = useTranslation()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const labelKey = props.labelKey ?? 'Clear chat history'

  const handleClearMessages = () => {
    props.onClearMessages?.()
    setConfirmOpen(false)
    toast.success(t(props.successKey ?? 'Conversation cleared'))
  }

  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={
            <PromptInputButton
              aria-label={t(labelKey)}
              className='text-muted-foreground hover:text-destructive hover:bg-destructive/10 font-medium'
              disabled={
                props.disabled || !props.hasMessages || !props.onClearMessages
              }
              onClick={() => setConfirmOpen(true)}
              variant='ghost'
            >
              <Trash2Icon size={16} />
            </PromptInputButton>
          }
        />
        <TooltipContent>
          <p>{t(labelKey)}</p>
        </TooltipContent>
      </Tooltip>

      <ConfirmDialog
        destructive
        desc={t(
          props.descriptionKey ??
            'All playground messages saved in this browser will be removed. This cannot be undone.'
        )}
        confirmText={t('Clear')}
        handleConfirm={handleClearMessages}
        onOpenChange={setConfirmOpen}
        open={confirmOpen}
        title={t(props.titleKey ?? 'Clear chat history?')}
      />
    </>
  )
}

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
import { useState, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { getSelf } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { redeemTopupCode } from '../api'
import type { RedemptionResponse, RedemptionResult } from '../types'

// ============================================================================
// Redemption Hook
// ============================================================================

function resolveRedemptionResult(
  response: RedemptionResponse
): RedemptionResult | null {
  if (response.redemption_result) {
    return response.redemption_result
  }
  if (typeof response.data === 'number') {
    return {
      benefit_type: 'quota',
      quota: response.data,
    }
  }
  if (response.data && typeof response.data === 'object') {
    return response.data as RedemptionResult
  }
  return null
}

export function useRedemption() {
  const [redeeming, setRedeeming] = useState(false)

  const redeemCode = useCallback(async (code: string): Promise<boolean> => {
    if (!code || code.trim() === '') {
      toast.error(i18next.t('Please enter a redemption code'))
      return false
    }

    try {
      setRedeeming(true)
      const response = await redeemTopupCode({ key: code })

      const result = response.success ? resolveRedemptionResult(response) : null

      if (response.success && result) {
        if (result.benefit_type === 'subscription') {
          const plan =
            result.subscription_plan_title ||
            `${i18next.t('Subscription')} #${result.subscription_plan_id || '-'}`
          toast.success(`${i18next.t('Added successfully')}: ${plan}`)
        } else {
          toast.success(
            i18next.t('Redemption successful! Added: {{quota}}', {
              quota: formatQuota(result.quota || 0),
            })
          )
        }
        await getSelf()
        return true
      }

      toast.error(response.message || i18next.t('Redemption failed'))
      return false
    } catch (_error) {
      toast.error(i18next.t('Redemption failed'))
      return false
    } finally {
      setRedeeming(false)
    }
  }, [])

  return {
    redeeming,
    redeemCode,
  }
}

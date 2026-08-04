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
import i18next from "i18next";
import { useState, useCallback } from "react";
import { toast } from "sonner";

import {
  calculateAmount,
  calculateStripeAmount,
  calculateWaffoAmount,
  calculateWaffoPancakeAmount,
  calculateInfiniAmount,
  requestPayment,
  requestStripePayment,
  requestInfiniPayment,
  isApiSuccess,
} from "../api";
import {
  isStripePayment,
  isWaffoPayment,
  isWaffoPancakePayment,
  isInfiniPayment,
  submitPaymentForm,
} from "../lib";
import type { AmountRequest, AmountResponse } from "../types";

// ============================================================================
// Payment Hook
// ============================================================================

type AmountCalculator = (request: AmountRequest) => Promise<AmountResponse>;

export interface PaymentAmountCalculators {
  regular: AmountCalculator;
  stripe: AmountCalculator;
  waffo: AmountCalculator;
  waffoPancake: AmountCalculator;
  infini: AmountCalculator;
}

const defaultPaymentAmountCalculators: PaymentAmountCalculators = {
  regular: calculateAmount,
  stripe: calculateStripeAmount,
  waffo: calculateWaffoAmount,
  waffoPancake: calculateWaffoPancakeAmount,
  infini: calculateInfiniAmount,
};

export async function requestPaymentAmount(
  topupAmount: number,
  paymentType: string,
  calculators: PaymentAmountCalculators = defaultPaymentAmountCalculators,
): Promise<number> {
  let calculator = calculators.regular;
  if (isStripePayment(paymentType)) {
    calculator = calculators.stripe;
  } else if (isWaffoPayment(paymentType)) {
    calculator = calculators.waffo;
  } else if (isWaffoPancakePayment(paymentType)) {
    calculator = calculators.waffoPancake;
  } else if (isInfiniPayment(paymentType)) {
    calculator = calculators.infini;
  }

  const response = await calculator({ amount: topupAmount });
  if (!isApiSuccess(response) || !response.data) {
    return 0;
  }

  return Number.parseFloat(response.data);
}

export function usePayment() {
  const [amount, setAmount] = useState<number>(0);
  const [calculating, setCalculating] = useState(false);
  const [processing, setProcessing] = useState(false);

  // Calculate payment amount
  const calculatePaymentAmount = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setCalculating(true);
        const calculatedAmount = await requestPaymentAmount(
          topupAmount,
          paymentType,
        );
        setAmount(calculatedAmount);
        return calculatedAmount;
      } catch {
        setAmount(0);
        return 0;
      } finally {
        setCalculating(false);
      }
    },
    [],
  );

  // Process payment
  const processPayment = useCallback(
    async (topupAmount: number, paymentType: string) => {
      let paymentWindow: Window | null = null;
      try {
        setProcessing(true);

        const isStripe = isStripePayment(paymentType);
        const isInfini = isInfiniPayment(paymentType);
        if (isInfini) {
          paymentWindow = window.open("", "_blank");
          if (paymentWindow) paymentWindow.opener = null;
        }
        const amount = Math.floor(topupAmount);

        let response;
        if (isStripe) {
          response = await requestStripePayment({
            amount,
            payment_method: "stripe",
          });
        } else if (isInfini) {
          response = await requestInfiniPayment({ amount });
        } else {
          response = await requestPayment({
            amount,
            payment_method: paymentType,
          });
        }

        if (!isApiSuccess(response)) {
          paymentWindow?.close();
          toast.error(response.message || i18next.t("Payment request failed"));
          return false;
        }

        // Handle Stripe payment
        if (isStripe) {
          const stripeResponse = response as Awaited<
            ReturnType<typeof requestStripePayment>
          >;
          if (stripeResponse.data?.pay_link) {
            window.open(stripeResponse.data.pay_link, "_blank");
            toast.success(i18next.t("Redirecting to payment page..."));
            return true;
          }
        }

        if (isInfini) {
          const infiniResponse = response as Awaited<
            ReturnType<typeof requestInfiniPayment>
          >;
          if (infiniResponse.data?.checkout_url) {
            const checkoutURL = infiniResponse.data.checkout_url;
            if (paymentWindow) {
              paymentWindow.location.href = checkoutURL;
            } else {
              window.location.href = checkoutURL;
            }
            toast.success(i18next.t("Redirecting to payment page..."));
            return true;
          }
        }

        // Handle non-Stripe payment
        if (!isStripe && !isInfini && response.data) {
          const url = (response as unknown as { url?: string }).url;
          if (url) {
            submitPaymentForm(url, response.data);
            toast.success(i18next.t("Redirecting to payment page..."));
            return true;
          }
        }

        paymentWindow?.close();
        return false;
      } catch {
        paymentWindow?.close();
        toast.error(i18next.t("Payment request failed"));
        return false;
      } finally {
        setProcessing(false);
      }
    },
    [],
  );

  return {
    amount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
    setAmount,
  };
}

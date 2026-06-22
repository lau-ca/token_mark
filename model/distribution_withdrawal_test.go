package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCalculateTopUpQuotaMatchesWalletTopUpRules(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	defer func() {
		common.QuotaPerUnit = oldQuotaPerUnit
	}()

	cases := []struct {
		name   string
		topUp  TopUp
		expect int64
	}{
		{
			name: "epay amount is display amount",
			topUp: TopUp{
				Amount:          2,
				Status:          common.TopUpStatusSuccess,
				PaymentProvider: PaymentProviderEpay,
			},
			expect: 1000000,
		},
		{
			name: "stripe money is charged amount",
			topUp: TopUp{
				Money:           3,
				Status:          common.TopUpStatusSuccess,
				PaymentProvider: PaymentProviderStripe,
			},
			expect: 1500000,
		},
		{
			name: "creem amount is already quota",
			topUp: TopUp{
				Amount:          6000,
				Status:          common.TopUpStatusSuccess,
				PaymentProvider: PaymentProviderCreem,
			},
			expect: 6000,
		},
		{
			name: "pending topup is ignored",
			topUp: TopUp{
				Amount:          2,
				Status:          common.TopUpStatusPending,
				PaymentProvider: PaymentProviderEpay,
			},
			expect: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expect, calculateTopUpQuota(tc.topUp))
		})
	}
}

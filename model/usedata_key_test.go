package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetUserKeyQuotaDataIncludesEveryOwnedKeyAndDeletedUsage(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&Token{
		Id:           11,
		UserId:       1,
		Key:          "primary-secret-key",
		Name:         "primary",
		Status:       common.TokenStatusEnabled,
		AccessedTime: 1900,
	}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:           12,
		UserId:       1,
		Key:          "backup-secret-key",
		Name:         "backup",
		Status:       common.TokenStatusDisabled,
		AccessedTime: 0,
	}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:           13,
		UserId:       1,
		Key:          "deleted-secret-key",
		Name:         "deleted",
		Status:       common.TokenStatusEnabled,
		AccessedTime: 1600,
	}).Error)
	require.NoError(t, DB.Delete(&Token{Id: 13}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:     21,
		UserId: 2,
		Key:    "other-user-key",
		Name:   "other",
		Status: common.TokenStatusEnabled,
	}).Error)

	rows := []QuotaData{
		{UserID: 1, TokenID: 11, CreatedAt: 1000, Count: 2, Quota: 100, TokenUsed: 40},
		{UserID: 1, TokenID: 11, CreatedAt: 1000, Count: 1, Quota: 50, TokenUsed: 20},
		{UserID: 1, TokenID: 13, CreatedAt: 1200, Count: 1, Quota: 25, TokenUsed: 10},
		{UserID: 1, TokenID: 11, CreatedAt: 3000, Count: 99, Quota: 999, TokenUsed: 999},
		{UserID: 2, TokenID: 21, CreatedAt: 1100, Count: 3, Quota: 70, TokenUsed: 30},
	}
	require.NoError(t, DB.Create(&rows).Error)

	data, err := GetUserKeyQuotaData(1, 900, 2000)
	require.NoError(t, err)
	require.Equal(t, []*KeyQuotaData{
		{
			TokenID:      11,
			TokenName:    "primary",
			MaskedKey:    MaskTokenKey("primary-secret-key"),
			TokenStatus:  common.TokenStatusEnabled,
			AccessedTime: 1900,
			CreatedAt:    1000,
			Count:        3,
			Quota:        150,
			TokenUsed:    60,
		},
		{
			TokenID:      12,
			TokenName:    "backup",
			MaskedKey:    MaskTokenKey("backup-secret-key"),
			TokenStatus:  common.TokenStatusDisabled,
			AccessedTime: 0,
		},
		{
			TokenID:   13,
			CreatedAt: 1200,
			Count:     1,
			Quota:     25,
			TokenUsed: 10,
			Deleted:   true,
		},
	}, data)
}

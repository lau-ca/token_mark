package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserKeyUsageExportAggregatesOwnedKeysAndModels(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&Token{
		Id:           11,
		UserId:       1,
		Key:          "primary-secret-key",
		Name:         "primary",
		Status:       common.TokenStatusEnabled,
		AccessedTime: 2500,
	}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:     12,
		UserId: 1,
		Key:    "backup-secret-key",
		Name:   "backup",
		Status: common.TokenStatusDisabled,
	}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:     13,
		UserId: 1,
		Key:    "deleted-secret-key",
		Name:   "deleted",
		Status: common.TokenStatusEnabled,
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
		{UserID: 1, TokenID: 11, ModelName: "gpt-5", Count: 2, TokenUsed: 140, Quota: 300, CreatedAt: 1100},
		{UserID: 1, TokenID: 11, ModelName: "gpt-5", Count: 1, TokenUsed: 60, Quota: 100, CreatedAt: 1300},
		{UserID: 1, TokenID: 11, ModelName: "claude-opus", Count: 1, TokenUsed: 100, Quota: 200, CreatedAt: 1200},
		{UserID: 1, TokenID: 13, ModelName: "gemini-2.5-pro", Count: 1, TokenUsed: 35, Quota: 50, CreatedAt: 1400},
		{UserID: 1, TokenID: 11, ModelName: "outside-range", Count: 99, TokenUsed: 999, Quota: 999, CreatedAt: 3000},
		{UserID: 2, TokenID: 21, ModelName: "gpt-5", Count: 99, TokenUsed: 999, Quota: 999, CreatedAt: 1250},
	}
	require.NoError(t, DB.Create(&rows).Error)

	data, err := GetUserKeyUsageExport(1, 1000, 2000)
	require.NoError(t, err)
	require.Len(t, data.Keys, 3)
	require.Len(t, data.Models, 3)

	assert.Equal(t, []KeyUsageExportKey{
		{
			TokenID:      11,
			TokenName:    "primary",
			MaskedKey:    MaskTokenKey("primary-secret-key"),
			TokenStatus:  common.TokenStatusEnabled,
			RequestCount: 4,
			TotalTokens:  300,
			Quota:        600,
			ModelCount:   2,
			LastUsedAt:   1300,
		},
		{
			TokenID:      13,
			RequestCount: 1,
			TotalTokens:  35,
			Quota:        50,
			ModelCount:   1,
			LastUsedAt:   1400,
			Deleted:      true,
		},
		{
			TokenID:     12,
			TokenName:   "backup",
			MaskedKey:   MaskTokenKey("backup-secret-key"),
			TokenStatus: common.TokenStatusDisabled,
		},
	}, data.Keys)

	assert.Equal(t, []KeyUsageExportModel{
		{
			TokenID:      11,
			TokenName:    "primary",
			MaskedKey:    MaskTokenKey("primary-secret-key"),
			ModelName:    "gpt-5",
			RequestCount: 3,
			TotalTokens:  200,
			Quota:        400,
			LastUsedAt:   1300,
		},
		{
			TokenID:      11,
			TokenName:    "primary",
			MaskedKey:    MaskTokenKey("primary-secret-key"),
			ModelName:    "claude-opus",
			RequestCount: 1,
			TotalTokens:  100,
			Quota:        200,
			LastUsedAt:   1200,
		},
		{
			TokenID:      13,
			ModelName:    "gemini-2.5-pro",
			RequestCount: 1,
			TotalTokens:  35,
			Quota:        50,
			LastUsedAt:   1400,
			Deleted:      true,
		},
	}, data.Models)
}

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

	logs := []Log{
		{UserId: 1, Type: LogTypeConsume, TokenId: 11, TokenName: "old-primary", ModelName: "gpt-5", PromptTokens: 100, CompletionTokens: 40, Quota: 300, CreatedAt: 1100},
		{UserId: 1, Type: LogTypeConsume, TokenId: 11, TokenName: "primary", ModelName: "gpt-5", PromptTokens: 50, CompletionTokens: 10, Quota: 100, CreatedAt: 1300},
		{UserId: 1, Type: LogTypeConsume, TokenId: 11, TokenName: "primary", ModelName: "claude-opus", PromptTokens: 80, CompletionTokens: 20, Quota: 200, CreatedAt: 1200},
		{UserId: 1, Type: LogTypeConsume, TokenId: 13, TokenName: "deleted", ModelName: "gemini-2.5-pro", PromptTokens: 30, CompletionTokens: 5, Quota: 50, CreatedAt: 1400},
		{UserId: 1, Type: LogTypeError, TokenId: 11, TokenName: "primary", ModelName: "ignored-error", PromptTokens: 999, CompletionTokens: 999, Quota: 999, CreatedAt: 1500},
		{UserId: 1, Type: LogTypeConsume, TokenId: 11, TokenName: "primary", ModelName: "outside-range", PromptTokens: 999, CompletionTokens: 999, Quota: 999, CreatedAt: 3000},
		{UserId: 2, Type: LogTypeConsume, TokenId: 21, TokenName: "other", ModelName: "gpt-5", PromptTokens: 999, CompletionTokens: 999, Quota: 999, CreatedAt: 1250},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	data, err := GetUserKeyUsageExport(1, 1000, 2000)
	require.NoError(t, err)
	require.Len(t, data.Keys, 3)
	require.Len(t, data.Models, 3)

	assert.Equal(t, []KeyUsageExportKey{
		{
			TokenID:          11,
			TokenName:        "primary",
			MaskedKey:        MaskTokenKey("primary-secret-key"),
			TokenStatus:      common.TokenStatusEnabled,
			RequestCount:     3,
			PromptTokens:     230,
			CompletionTokens: 70,
			TotalTokens:      300,
			Quota:            600,
			ModelCount:       2,
			LastUsedAt:       1300,
		},
		{
			TokenID:          13,
			TokenName:        "deleted",
			RequestCount:     1,
			PromptTokens:     30,
			CompletionTokens: 5,
			TotalTokens:      35,
			Quota:            50,
			ModelCount:       1,
			LastUsedAt:       1400,
			Deleted:          true,
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
			TokenID:          11,
			TokenName:        "primary",
			MaskedKey:        MaskTokenKey("primary-secret-key"),
			ModelName:        "gpt-5",
			RequestCount:     2,
			PromptTokens:     150,
			CompletionTokens: 50,
			TotalTokens:      200,
			Quota:            400,
			LastUsedAt:       1300,
		},
		{
			TokenID:          11,
			TokenName:        "primary",
			MaskedKey:        MaskTokenKey("primary-secret-key"),
			ModelName:        "claude-opus",
			RequestCount:     1,
			PromptTokens:     80,
			CompletionTokens: 20,
			TotalTokens:      100,
			Quota:            200,
			LastUsedAt:       1200,
		},
		{
			TokenID:          13,
			TokenName:        "deleted",
			ModelName:        "gemini-2.5-pro",
			RequestCount:     1,
			PromptTokens:     30,
			CompletionTokens: 5,
			TotalTokens:      35,
			Quota:            50,
			LastUsedAt:       1400,
			Deleted:          true,
		},
	}, data.Models)
}

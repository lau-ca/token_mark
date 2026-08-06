package model

import "sort"

type KeyUsageExportKey struct {
	TokenID      int    `json:"token_id"`
	TokenName    string `json:"token_name"`
	MaskedKey    string `json:"masked_key"`
	TokenStatus  int    `json:"token_status"`
	RequestCount int    `json:"request_count"`
	TotalTokens  int    `json:"total_tokens"`
	Quota        int    `json:"quota"`
	ModelCount   int    `json:"model_count"`
	LastUsedAt   int64  `json:"last_used_at"`
	Deleted      bool   `json:"deleted"`
}

type KeyUsageExportModel struct {
	TokenID      int    `json:"token_id" gorm:"column:token_id"`
	TokenName    string `json:"token_name" gorm:"-"`
	MaskedKey    string `json:"masked_key" gorm:"-"`
	ModelName    string `json:"model_name" gorm:"column:model_name"`
	RequestCount int    `json:"request_count" gorm:"column:request_count"`
	TotalTokens  int    `json:"total_tokens" gorm:"column:total_tokens"`
	Quota        int    `json:"quota" gorm:"column:quota"`
	LastUsedAt   int64  `json:"last_used_at" gorm:"column:last_used_at"`
	Deleted      bool   `json:"deleted" gorm:"-"`
}

type KeyUsageExportData struct {
	Keys   []KeyUsageExportKey   `json:"keys"`
	Models []KeyUsageExportModel `json:"models"`
}

func GetUserKeyUsageExport(userID int, startTime int64, endTime int64) (*KeyUsageExportData, error) {
	models := make([]KeyUsageExportModel, 0)
	err := DB.Table("quota_data").
		Select("token_id, model_name, COALESCE(SUM(count), 0) AS request_count, COALESCE(SUM(token_used), 0) AS total_tokens, COALESCE(SUM(quota), 0) AS quota, MAX(created_at) AS last_used_at").
		Where("user_id = ? AND token_id > 0", userID).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Group("token_id, model_name").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	var tokens []Token
	if err := DB.Model(&Token{}).
		Select("id, user_id, key, name, status, accessed_time").
		Where("user_id = ?", userID).
		Find(&tokens).Error; err != nil {
		return nil, err
	}

	tokenByID := make(map[int]Token, len(tokens))
	for _, token := range tokens {
		tokenByID[token.Id] = token
	}

	keyByID := make(map[int]*KeyUsageExportKey, len(tokens)+len(models))
	for index := range models {
		row := &models[index]
		if token, exists := tokenByID[row.TokenID]; exists {
			row.TokenName = token.Name
			row.MaskedKey = token.GetMaskedKey()
		} else {
			row.Deleted = true
		}

		key := keyByID[row.TokenID]
		if key == nil {
			key = &KeyUsageExportKey{
				TokenID:    row.TokenID,
				TokenName:  row.TokenName,
				MaskedKey:  row.MaskedKey,
				LastUsedAt: row.LastUsedAt,
				Deleted:    row.Deleted,
			}
			if token, exists := tokenByID[row.TokenID]; exists {
				key.TokenStatus = token.Status
				if token.AccessedTime >= startTime && token.AccessedTime <= endTime {
					key.LastUsedAt = token.AccessedTime
				}
			}
			keyByID[row.TokenID] = key
		}
		key.RequestCount += row.RequestCount
		key.TotalTokens += row.TotalTokens
		key.Quota += row.Quota
		if row.ModelName != "" {
			key.ModelCount++
		}
		if row.LastUsedAt > key.LastUsedAt {
			key.LastUsedAt = row.LastUsedAt
		}
	}

	for _, token := range tokens {
		if _, exists := keyByID[token.Id]; exists {
			continue
		}
		keyByID[token.Id] = &KeyUsageExportKey{
			TokenID:     token.Id,
			TokenName:   token.Name,
			MaskedKey:   token.GetMaskedKey(),
			TokenStatus: token.Status,
		}
	}

	keys := make([]KeyUsageExportKey, 0, len(keyByID))
	for _, key := range keyByID {
		keys = append(keys, *key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Quota != keys[j].Quota {
			return keys[i].Quota > keys[j].Quota
		}
		return keys[i].TokenID < keys[j].TokenID
	})
	sort.Slice(models, func(i, j int) bool {
		if models[i].TokenID != models[j].TokenID {
			return models[i].TokenID < models[j].TokenID
		}
		if models[i].Quota != models[j].Quota {
			return models[i].Quota > models[j].Quota
		}
		return models[i].ModelName < models[j].ModelName
	})

	return &KeyUsageExportData{Keys: keys, Models: models}, nil
}

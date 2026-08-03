package model

import "sort"

type KeyQuotaData struct {
	TokenID      int    `json:"token_id" gorm:"column:token_id"`
	TokenName    string `json:"token_name" gorm:"-"`
	MaskedKey    string `json:"masked_key" gorm:"-"`
	TokenStatus  int    `json:"token_status" gorm:"-"`
	AccessedTime int64  `json:"accessed_time" gorm:"-"`
	CreatedAt    int64  `json:"created_at" gorm:"column:created_at"`
	TokenUsed    int    `json:"token_used" gorm:"column:token_used"`
	Count        int    `json:"count" gorm:"column:count"`
	Quota        int    `json:"quota" gorm:"column:quota"`
	Deleted      bool   `json:"deleted" gorm:"-"`
}

func GetUserKeyQuotaData(userID int, startTime int64, endTime int64) ([]*KeyQuotaData, error) {
	rows := make([]*KeyQuotaData, 0)
	err := DB.Table("quota_data").
		Select("token_id, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("user_id = ? AND token_id > 0", userID).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Group("token_id, created_at").
		Find(&rows).Error
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
	usedTokenIDs := make(map[int]struct{}, len(rows))
	for _, token := range tokens {
		tokenByID[token.Id] = token
	}
	for _, row := range rows {
		usedTokenIDs[row.TokenID] = struct{}{}
		token, exists := tokenByID[row.TokenID]
		if !exists {
			row.Deleted = true
			continue
		}
		row.TokenName = token.Name
		row.MaskedKey = token.GetMaskedKey()
		row.TokenStatus = token.Status
		row.AccessedTime = token.AccessedTime
	}

	for _, token := range tokens {
		if _, exists := usedTokenIDs[token.Id]; exists {
			continue
		}
		rows = append(rows, &KeyQuotaData{
			TokenID:      token.Id,
			TokenName:    token.Name,
			MaskedKey:    token.GetMaskedKey(),
			TokenStatus:  token.Status,
			AccessedTime: token.AccessedTime,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TokenID != rows[j].TokenID {
			return rows[i].TokenID < rows[j].TokenID
		}
		return rows[i].CreatedAt < rows[j].CreatedAt
	})
	return rows, nil
}

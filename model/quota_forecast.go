package model

type QuotaForecastUser struct {
	UserID    int   `gorm:"column:user_id"`
	Quota     int   `gorm:"column:quota"`
	CreatedAt int64 `gorm:"column:created_at"`
}

type QuotaForecastUsage struct {
	UserID    int   `gorm:"column:user_id"`
	CreatedAt int64 `gorm:"column:created_at"`
	Quota     int64 `gorm:"column:quota"`
}

func GetQuotaForecastUsers(userIDs []int) ([]QuotaForecastUser, error) {
	users := make([]QuotaForecastUser, 0, len(userIDs))
	if len(userIDs) == 0 {
		return users, nil
	}
	err := DB.Model(&User{}).
		Select("id AS user_id, quota, created_at").
		Where("id IN ?", userIDs).
		Find(&users).Error
	return users, err
}

func GetQuotaForecastUsage(userIDs []int, startTimestamp int64, endTimestamp int64) ([]QuotaForecastUsage, error) {
	usage := make([]QuotaForecastUsage, 0)
	if len(userIDs) == 0 {
		return usage, nil
	}
	err := DB.Table("quota_data").
		Select("user_id, created_at, SUM(quota) AS quota").
		Where("user_id IN ?", userIDs).
		Where("created_at >= ? AND created_at < ?", startTimestamp, endTimestamp).
		Group("user_id, created_at").
		Find(&usage).Error
	return usage, err
}

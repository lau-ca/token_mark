package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
)

var beijingLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type ChannelHealthCounts struct {
	ChannelID       int   `json:"channel_id" gorm:"column:channel_id"`
	SuccessCount    int64 `json:"success_count" gorm:"column:success_count"`
	ErrorCount      int64 `json:"error_count" gorm:"column:error_count"`
	LastSuccessID   int64 `json:"-" gorm:"column:last_success_id"`
	LastErrorID     int64 `json:"-" gorm:"column:last_error_id"`
	LastSuccessTime int64 `json:"-" gorm:"column:last_success_time"`
	LastErrorTime   int64 `json:"-" gorm:"column:last_error_time"`
}

func BeijingDayRange(now time.Time) (string, int64, int64) {
	localNow := now.In(beijingLocation)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, beijingLocation)
	return start.Format("2006-01-02"), start.Unix(), now.Unix() + 1
}

func GetChannelHealthCounts(channelIDs []int, startTimestamp, endTimestamp int64) (map[int]ChannelHealthCounts, error) {
	result := make(map[int]ChannelHealthCounts, len(channelIDs))
	if len(channelIDs) == 0 {
		return result, nil
	}
	rows := make([]ChannelHealthCounts, 0, len(channelIDs))
	err := LOG_DB.Model(&Log{}).
		Select(`channel_id,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_count,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS error_count,
			MAX(CASE WHEN type = ? THEN id ELSE 0 END) AS last_success_id,
			MAX(CASE WHEN type = ? THEN id ELSE 0 END) AS last_error_id,
			MAX(CASE WHEN type = ? THEN created_at ELSE 0 END) AS last_success_time,
			MAX(CASE WHEN type = ? THEN created_at ELSE 0 END) AS last_error_time`,
			LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError).
		Where("channel_id IN ?", channelIDs).
		Where("channel_id <> 0").
		Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
		Where("created_at >= ? AND created_at < ?", startTimestamp, endTimestamp).
		Group("channel_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ChannelID] = row
	}
	return result, nil
}

func (channel *Channel) UpdateHealthSnapshot() error {
	return DB.Model(channel).
		Select("health_status", "health_error_rate", "health_success_count", "health_error_count", "health_total_count", "health_date", "health_updated_time", "health_last_call_status", "health_last_call_time").
		Updates(Channel{
			HealthStatus:         channel.HealthStatus,
			HealthErrorRate:      channel.HealthErrorRate,
			HealthSuccessCount:   channel.HealthSuccessCount,
			HealthErrorCount:     channel.HealthErrorCount,
			HealthTotalCount:     channel.HealthTotalCount,
			HealthDate:           channel.HealthDate,
			HealthUpdatedTime:    channel.HealthUpdatedTime,
			HealthLastCallStatus: channel.HealthLastCallStatus,
			HealthLastCallTime:   channel.HealthLastCallTime,
		}).Error
}

func NewUnknownChannelHealthSnapshot(channel *Channel, date string) {
	channel.HealthStatus = "unknown"
	channel.HealthErrorRate = 0
	channel.HealthSuccessCount = 0
	channel.HealthErrorCount = 0
	channel.HealthTotalCount = 0
	channel.HealthDate = date
	channel.HealthUpdatedTime = common.GetTimestamp()
	channel.HealthLastCallStatus = "none"
	channel.HealthLastCallTime = 0
}

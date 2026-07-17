package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelHealthBeijingDayRange(t *testing.T) {
	now := time.Date(2026, 7, 17, 16, 30, 0, 0, time.UTC)
	date, start, end := BeijingDayRange(now)

	assert.Equal(t, "2026-07-18", date)
	assert.Equal(t, time.Date(2026, 7, 17, 16, 0, 0, 0, time.UTC).Unix(), start)
	assert.Equal(t, now.Unix()+1, end)
}

func TestGetChannelHealthCountsIncludesLatestCall(t *testing.T) {
	originalLogDB := LOG_DB
	t.Cleanup(func() {
		LOG_DB = originalLogDB
	})

	db, err := gorm.Open(sqlite.Open("file:channel-health?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db

	logs := []Log{
		{Id: 1, ChannelId: 10, Type: LogTypeConsume, CreatedAt: 100},
		{Id: 2, ChannelId: 10, Type: LogTypeError, CreatedAt: 110},
		{Id: 3, ChannelId: 10, Type: LogTypeConsume, CreatedAt: 110},
		{Id: 4, ChannelId: 10, Type: LogTypeManage, CreatedAt: 120},
		{Id: 5, ChannelId: 10, Type: LogTypeError, CreatedAt: 200},
		{Id: 6, ChannelId: 20, Type: LogTypeError, CreatedAt: 120},
	}
	require.NoError(t, db.Create(&logs).Error)

	counts, err := GetChannelHealthCounts([]int{10, 20}, 100, 200)
	require.NoError(t, err)

	assert.Equal(t, int64(2), counts[10].SuccessCount)
	assert.Equal(t, int64(1), counts[10].ErrorCount)
	assert.Equal(t, int64(3), counts[10].LastSuccessID)
	assert.Equal(t, int64(2), counts[10].LastErrorID)
	assert.Equal(t, int64(110), counts[10].LastSuccessTime)
	assert.Equal(t, int64(1), counts[20].ErrorCount)
	assert.Equal(t, int64(6), counts[20].LastErrorID)
	assert.Equal(t, int64(120), counts[20].LastErrorTime)
}

package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChannelHealthBeijingDayRange(t *testing.T) {
	now := time.Date(2026, 7, 17, 16, 30, 0, 0, time.UTC)
	date, start, end := BeijingDayRange(now)

	assert.Equal(t, "2026-07-18", date)
	assert.Equal(t, time.Date(2026, 7, 17, 16, 0, 0, 0, time.UTC).Unix(), start)
	assert.Equal(t, now.Unix()+1, end)
}

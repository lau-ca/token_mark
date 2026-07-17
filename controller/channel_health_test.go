package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestClassifyChannelHealth(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		errorRate float64
		want      string
	}{
		{name: "insufficient samples", total: 19, errorRate: 100, want: channelHealthUnknown},
		{name: "healthy", total: 20, errorRate: 1.99, want: channelHealthHealthy},
		{name: "warning boundary", total: 20, errorRate: 2, want: channelHealthWarning},
		{name: "warning", total: 100, errorRate: 9.99, want: channelHealthWarning},
		{name: "critical boundary", total: 100, errorRate: 10, want: channelHealthCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, classifyChannelHealth(tt.total, tt.errorRate))
		})
	}
}

func TestSetChannelLastCallSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		counts     model.ChannelHealthCounts
		wantStatus string
		wantTime   int64
	}{
		{name: "no calls", wantStatus: channelLastCallNone},
		{
			name: "latest call succeeded",
			counts: model.ChannelHealthCounts{
				LastSuccessID:   12,
				LastErrorID:     11,
				LastSuccessTime: 100,
				LastErrorTime:   99,
			},
			wantStatus: channelLastCallSuccess,
			wantTime:   100,
		},
		{
			name: "latest call failed",
			counts: model.ChannelHealthCounts{
				LastSuccessID: 10,
				LastErrorID:   13,
				LastErrorTime: 120,
			},
			wantStatus: channelLastCallError,
			wantTime:   120,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &model.Channel{}
			setChannelLastCallSnapshot(channel, tt.counts)
			assert.Equal(t, tt.wantStatus, channel.HealthLastCallStatus)
			assert.Equal(t, tt.wantTime, channel.HealthLastCallTime)
		})
	}
}

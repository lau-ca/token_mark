package controller

import (
	"testing"

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

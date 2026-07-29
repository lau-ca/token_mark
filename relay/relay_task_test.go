package relay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsOpenAIVideoRequest(t *testing.T) {
	tests := []struct {
		requestURI string
		want       bool
	}{
		{requestURI: "/v1/videos/task_123", want: true},
		{requestURI: "/pg/videos/task_123", want: true},
		{requestURI: "/pg/videos/task_123?refresh=true", want: true},
		{requestURI: "/v1/video/generations/task_123", want: false},
	}

	for _, test := range tests {
		t.Run(test.requestURI, func(t *testing.T) {
			assert.Equal(t, test.want, isOpenAIVideoRequest(test.requestURI))
		})
	}
}

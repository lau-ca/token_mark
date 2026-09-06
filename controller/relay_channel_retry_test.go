package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetChannelRetryTimes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		settings dto.ChannelSettings
		expected int
	}{
		{name: "disabled"},
		{name: "configured", settings: dto.ChannelSettings{RetryTimes: 2}, expected: 2},
		{name: "runtime clamp", settings: dto.ChannelSettings{RetryTimes: dto.MaxChannelRetryTimes + 5}, expected: dto.MaxChannelRetryTimes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(c, constant.ContextKeyChannelSetting, tt.settings)
			assert.Equal(t, tt.expected, getChannelRetryTimes(c))
		})
	}
}

func TestShouldRetrySameChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		err       *types.NewAPIError
		remaining int
		written   bool
		canceled  bool
		expected  bool
	}{
		{
			name:      "retryable server error",
			err:       types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponse, http.StatusInternalServerError),
			remaining: 1,
			expected:  true,
		},
		{
			name:      "bad request",
			err:       types.NewErrorWithStatusCode(errors.New("invalid request"), types.ErrorCodeInvalidRequest, http.StatusBadRequest),
			remaining: 1,
			expected:  true,
		},
		{
			name:      "no retries remaining",
			err:       types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponse, http.StatusInternalServerError),
			remaining: 0,
		},
		{
			name:      "skip retry marker does not block channel retry",
			err:       types.NewErrorWithStatusCode(errors.New("invalid request"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry()),
			remaining: 1,
			expected:  true,
		},
		{
			name:      "response already written",
			err:       types.NewErrorWithStatusCode(errors.New("stream interrupted"), types.ErrorCodeBadResponse, http.StatusInternalServerError),
			remaining: 1,
			written:   true,
		},
		{
			name:      "client canceled",
			err:       types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponse, http.StatusInternalServerError),
			remaining: 1,
			canceled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			if tt.canceled {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil).WithContext(ctx)
			}
			if tt.written {
				c.String(http.StatusOK, "partial")
			}
			assert.Equal(t, tt.expected, shouldRetrySameChannel(c, tt.err, tt.remaining))
		})
	}
}

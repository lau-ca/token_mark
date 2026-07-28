package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
)

func TestShouldRetryCompositeImage(t *testing.T) {
	tests := []struct {
		name       string
		err        *types.NewAPIError
		expression string
		expected   bool
	}{
		{
			name:       "configured rate limit retries",
			err:        types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests),
			expression: "429,500-599",
			expected:   true,
		},
		{
			name:       "client validation error stops",
			err:        types.NewErrorWithStatusCode(errors.New("bad request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest),
			expression: "429,500-599",
			expected:   false,
		},
		{
			name:       "server error retries",
			err:        types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable),
			expression: "429,500-599",
			expected:   true,
		},
		{
			name:       "transport error retries",
			err:        types.NewErrorWithStatusCode(errors.New("connection reset"), types.ErrorCodeDoRequestFailed, 0),
			expression: "429,500-599",
			expected:   true,
		},
		{
			name:       "explicit skip retry wins",
			err:        types.NewErrorWithStatusCode(errors.New("invalid request"), types.ErrorCodeInvalidRequest, http.StatusInternalServerError, types.ErrOptionWithSkipRetry()),
			expression: "500-599",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, shouldRetryCompositeImage(tt.err, tt.expression))
		})
	}
}

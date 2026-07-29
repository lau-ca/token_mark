package controller

import (
	"errors"
	"net/http"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type compositeBillingSpy struct {
	refundCount int
}

func (s *compositeBillingSpy) Settle(int) error {
	return nil
}

func (s *compositeBillingSpy) Refund(*gin.Context) {
	s.refundCount++
}

func (s *compositeBillingSpy) NeedsRefund() bool {
	return true
}

func (s *compositeBillingSpy) GetPreConsumedQuota() int {
	return 1
}

func (s *compositeBillingSpy) Reserve(int) error {
	return nil
}

func TestResetCompositeRelayAttempt(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		RetryIndex:  3,
		LastError:   types.NewError(errors.New("previous failure"), types.ErrorCodeDoRequestFailed),
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "mapped-model", IsModelMapped: true},
		RequestConversionChain: []types.RelayFormat{
			types.RelayFormatOpenAIImage,
		},
		FinalRequestRelayFormat: types.RelayFormatOpenAIImage,
	}

	resetCompositeRelayAttempt(relayInfo, 1)

	require.NotNil(t, relayInfo.ChannelMeta)
	assert.Equal(t, 1, relayInfo.RetryIndex)
	assert.Nil(t, relayInfo.LastError)
	assert.Empty(t, relayInfo.UpstreamModelName)
	assert.False(t, relayInfo.IsModelMapped)
	assert.Empty(t, relayInfo.RequestConversionChain)
	assert.Empty(t, relayInfo.FinalRequestRelayFormat)
}

func TestCompositePanicRefund(t *testing.T) {
	billing := &compositeBillingSpy{}
	relayInfo := &relaycommon.RelayInfo{Billing: billing}
	c, _ := gin.CreateTestContext(nil)

	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		func() {
			defer refundCompositeBillingOnPanic(c, relayInfo)
			panic("composite panic")
		}()
	}()

	assert.Equal(t, "composite panic", recovered)
	assert.Equal(t, 1, billing.refundCount)
}

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

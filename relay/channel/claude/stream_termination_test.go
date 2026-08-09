package claude

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamHandlerStopsAfterMessageStop(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})

	upstreamReader, upstreamWriter := io.Pipe()
	releaseWriter := make(chan struct{})
	t.Cleanup(func() {
		close(releaseWriter)
	})

	go func() {
		_, _ = fmt.Fprintln(upstreamWriter, `data: {"type":"message_start","message":{"id":"msg_1","model":"claude-opus-5","usage":{"input_tokens":2}}}`)
		_, _ = fmt.Fprintln(upstreamWriter, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`)
		_, _ = fmt.Fprintln(upstreamWriter, `data: {"type":"message_stop"}`)
		<-releaseWriter
		_ = upstreamWriter.Close()
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"},
	}
	resp := &http.Response{Body: upstreamReader}

	done := make(chan struct{})
	var usageErr error
	go func() {
		_, relayErr := ClaudeStreamHandler(c, resp, info)
		if relayErr != nil {
			usageErr = relayErr
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ClaudeStreamHandler did not stop after message_stop")
	}

	require.NoError(t, usageErr)
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Contains(t, recorder.Body.String(), "event: message_stop")
	assert.Contains(t, recorder.Body.String(), `data: {"type":"message_stop"}`)
	assert.False(t, strings.Contains(recorder.Body.String(), "[DONE]"))
}

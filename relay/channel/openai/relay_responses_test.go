package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupResponsesStreamTest(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5",
		},
	}
	return c, recorder, resp, info
}

func TestOaiResponsesStreamHandlerDoesNotAppendFailureAfterCompleted(t *testing.T) {
	body := `data: {"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}` + "\n\n"
	c, recorder, resp, info := setupResponsesStreamTest(t, body)

	usage, err := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.NotContains(t, recorder.Body.String(), "responses_stream_incomplete")
}

func TestOaiResponsesStreamHandlerAppendsFailureWhenTerminalEventMissing(t *testing.T) {
	body := `data: {"type":"response.created"}` + "\n\n"
	c, recorder, resp, info := setupResponsesStreamTest(t, body)

	usage, err := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	output := recorder.Body.String()
	require.Contains(t, output, "event: response.created")
	require.Contains(t, output, "event: response.failed")
	require.Contains(t, output, "responses_stream_incomplete")
}

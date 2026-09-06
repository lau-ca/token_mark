package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newImageTestContext(t *testing.T, body, contentType string, isStream bool) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{contentType}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		IsStream:    isStream,
	}
	return c, recorder, resp, info
}

func TestOpenaiImageDoResponseUsesInfoIsStream(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"created":1710000000,"data":[{"b64_json":"image"}]}`

	t.Run("non-stream response stays JSON", func(t *testing.T) {
		c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
		info.RelayMode = relayconstant.RelayModeImagesGenerations

		usage, err := (&Adaptor{}).DoResponse(c, resp, info)

		require.Nil(t, err)
		require.NotNil(t, usage)
		require.Equal(t, body, recorder.Body.String())
	})

	t.Run("stream response converts JSON to SSE", func(t *testing.T) {
		c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)
		info.RelayMode = relayconstant.RelayModeImagesGenerations

		usage, err := (&Adaptor{}).DoResponse(c, resp, info)

		require.Nil(t, err)
		require.NotNil(t, usage)
		require.Contains(t, recorder.Body.String(), `event: image_generation.completed`)
		require.Contains(t, recorder.Body.String(), `data: [DONE]`)
	})
}

// TestOpenaiImageStreamHandlerForwardsSSEAndUsage covers the core SSE path:
// chunks are forwarded with rebuilt event lines, usage is extracted and
// normalized (input_tokens -> prompt_tokens with details), and [DONE] is
// re-emitted to the client.
func TestOpenaiImageStreamHandlerForwardsSSEAndUsage(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`event: image_generation.partial_image`,
		`data: {"type":"image_generation.partial_image","b64_json":"partial"}`,
		``,
		`data: {"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7,"input_tokens_details":{"image_tokens":2,"text_tokens":1}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newImageTestContext(t, body, "text/event-stream", true)
	info.PriceData.UsePrice = true
	info.PriceData.AddOtherRatio("n", 3)

	usage, err := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 7, usage.TotalTokens)
	require.Equal(t, 2, usage.PromptTokensDetails.ImageTokens)
	require.Equal(t, 1, usage.PromptTokensDetails.TextTokens)
	require.Contains(t, recorder.Body.String(), `event: image_generation.partial_image`)
	require.Contains(t, recorder.Body.String(), `data: {"type":"image_generation.partial_image","b64_json":"partial"}`)
	require.Contains(t, recorder.Body.String(), `data: {"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7,"input_tokens_details":{"image_tokens":2,"text_tokens":1}}}`)
	require.Contains(t, recorder.Body.String(), `data: [DONE]`)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Equal(t, 3.0, info.PriceData.OtherRatios()["n"], "streams without completed events keep the requested count")
}

func TestOpenaiImageStreamHandlerUsesCompletedEventCount(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"partial"}`,
		``,
		`data: {"type":"image_generation.completed","b64_json":"first"}`,
		``,
		`data: {"type":"image_edit.completed","b64_json":"second","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	c, _, resp, info := newImageTestContext(t, body, "text/event-stream", true)
	info.PriceData.UsePrice = true
	info.PriceData.AddOtherRatio("n", 3)

	usage, err := OpenaiImageStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.Equal(t, 7, usage.TotalTokens)
	require.Equal(t, 2.0, info.PriceData.OtherRatios()["n"])
}

// blockingBody serves one SSE chunk, then blocks until Close (the scanner's
// cleanup) and returns EOF — keeping the upstream "open" while the client-side
// disconnect is simulated elsewhere.
type blockingBody struct {
	mu     sync.Mutex
	sent   bool
	chunk  []byte
	closed chan struct{}
}

func (b *blockingBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	if !b.sent {
		b.sent = true
		n := copy(p, b.chunk)
		b.mu.Unlock()
		return n, nil
	}
	b.mu.Unlock()
	<-b.closed
	return 0, io.EOF
}

func (b *blockingBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-b.closed:
	default:
		close(b.closed)
	}
	return nil
}

// cancelAfterWriter cancels the request context right after the payload
// containing needle has been written to the client, simulating a client that
// disconnects after receiving that event. Cancelling from the write side (not
// the upstream read side) makes the abort deterministic: the handler has
// already processed and counted the event when the disconnect fires.
type cancelAfterWriter struct {
	gin.ResponseWriter
	needle string
	cancel context.CancelFunc
	once   sync.Once
}

func (w *cancelAfterWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if strings.Contains(string(p), w.needle) {
		w.once.Do(w.cancel)
	}
	return n, err
}

func (w *cancelAfterWriter) WriteString(s string) (int, error) {
	n, err := io.WriteString(w.ResponseWriter, s)
	if strings.Contains(s, w.needle) {
		w.once.Do(w.cancel)
	}
	return n, err
}

func newDisconnectingImageStream(t *testing.T, sseBody, disconnectAfter string) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()
	c, recorder, resp, info := newImageTestContext(t, "", "text/event-stream", true)
	ctx, cancel := context.WithCancel(c.Request.Context())
	t.Cleanup(cancel)
	c.Request = c.Request.WithContext(ctx)
	c.Writer = &cancelAfterWriter{ResponseWriter: c.Writer, needle: disconnectAfter, cancel: cancel}
	resp.Body = &blockingBody{
		chunk:  []byte(sseBody),
		closed: make(chan struct{}),
	}
	return c, recorder, resp, info
}

// TestOpenaiImageStreamHandlerClientDisconnectKeepsRequestedCount guards the
// billing invariant: completed-event counting must not lower the charge when
// the client aborts the stream. Upstream already generated (and charged for)
// all requested images, so a disconnect after the first completed event keeps
// the requested n instead of dropping it to 1.
func TestOpenaiImageStreamHandlerClientDisconnectKeepsRequestedCount(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := "data: {\"type\":\"image_generation.completed\",\"b64_json\":\"first\"}\n\n"
	c, recorder, resp, info := newDisconnectingImageStream(t, body, "first")
	info.PriceData.UsePrice = true
	info.PriceData.AddOtherRatio("n", 3)

	usage, err := OpenaiImageStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.NotNil(t, info.StreamStatus)
	// A client abort surfaces as client_gone (main-loop ctx watch) or
	// handler_stop (failed client write); both must be treated as untrusted.
	require.Contains(t,
		[]relaycommon.StreamEndReason{relaycommon.StreamEndReasonClientGone, relaycommon.StreamEndReasonHandlerStop},
		info.StreamStatus.EndReason)
	require.Contains(t, recorder.Body.String(), `"b64_json":"first"`)
	require.Equal(t, 3.0, info.PriceData.OtherRatios()["n"], "client abort must not reduce the billed image count")
}

// TestOpenaiImageStreamHandlerClientDisconnectRaisesCount covers the other
// direction of the abort guard: when completed events already exceed the
// recorded n, the higher actual count is billed even though the client aborted.
func TestOpenaiImageStreamHandlerClientDisconnectRaisesCount(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"image_generation.completed","b64_json":"first"}`,
		``,
		`data: {"type":"image_generation.completed","b64_json":"second"}`,
		``,
		``,
	}, "\n")
	c, _, resp, info := newDisconnectingImageStream(t, body, "second")
	info.PriceData.UsePrice = true
	info.PriceData.AddOtherRatio("n", 1)

	usage, err := OpenaiImageStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.NotNil(t, info.StreamStatus)
	require.Contains(t,
		[]relaycommon.StreamEndReason{relaycommon.StreamEndReasonClientGone, relaycommon.StreamEndReasonHandlerStop},
		info.StreamStatus.EndReason)
	require.Equal(t, 2.0, info.PriceData.OtherRatios()["n"], "completed events beyond the recorded n must raise the charge even on abort")
}

// TestOpenaiImageStreamHandlerWrapsJSONResponse covers the non-SSE fallback:
// a JSON upstream response is wrapped into pseudo-SSE completed events.
func TestOpenaiImageStreamHandlerWrapsJSONResponse(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"created":1710000000,"data":[{"b64_json":"first","revised_prompt":"draw a cat"},{"b64_json":"second"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7,"input_tokens_details":{"image_tokens":2,"text_tokens":1}}}`

	c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)
	info.PriceData.UsePrice = true
	info.PriceData.AddOtherRatio("n", 3)

	usage, err := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 7, usage.TotalTokens)
	require.Equal(t, 2, usage.PromptTokensDetails.ImageTokens)
	require.Equal(t, 1, usage.PromptTokensDetails.TextTokens)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Empty(t, recorder.Header().Get("Content-Length"))
	require.Contains(t, recorder.Body.String(), `event: image_generation.completed`)
	require.Contains(t, recorder.Body.String(), `"type":"image_generation.completed"`)
	require.Contains(t, recorder.Body.String(), `"b64_json":"first"`)
	require.Contains(t, recorder.Body.String(), `"b64_json":"second"`)
	require.Contains(t, recorder.Body.String(), `"revised_prompt":"draw a cat"`)
	require.Contains(t, recorder.Body.String(), `data: [DONE]`)
	require.Equal(t, 2, strings.Count(recorder.Body.String(), `event: image_generation.completed`))
	require.Equal(t, 2.0, info.PriceData.OtherRatios()["n"])
}

func TestOpenaiImageHandlerUsesPositiveActualCountForFixedPrice(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	longImage := strings.Repeat("a", 4096)

	tests := []struct {
		name      string
		body      string
		usePrice  bool
		wantCount float64
	}{
		{
			name:      "fixed price uses data length",
			body:      `{"data":[{"b64_json":"` + longImage + `"},{"b64_json":"second"}]}`,
			usePrice:  true,
			wantCount: 2,
		},
		{
			name:      "empty data keeps requested count",
			body:      `{"data":[]}`,
			usePrice:  true,
			wantCount: 3,
		},
		{
			name:      "ratio billing ignores data length",
			body:      `{"data":[{"b64_json":"first"},{"b64_json":"second"}]}`,
			usePrice:  false,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder, resp, info := newImageTestContext(t, tt.body, "application/json", false)
			info.PriceData.UsePrice = tt.usePrice
			info.PriceData.AddOtherRatio("n", 3)

			_, err := OpenaiImageHandler(c, info, resp)

			require.Nil(t, err)
			require.Equal(t, tt.wantCount, info.PriceData.OtherRatios()["n"])
			require.Equal(t, tt.body, recorder.Body.String())
		})
	}
}

func TestOpenaiImageHandlerStripsChannelImageURLsWhenBase64Exists(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"created":1710000000,"data":[{"b64_json":"final","url":"https://upstream.example/image.png","_provider_image_url":"https://provider.example/image.png","revised_prompt":"draw a cat"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`

	for _, relayMode := range []int{relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits} {
		c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
		info.RelayMode = relayMode
		info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{ForceImageB64JSONNoURL: true}

		usage, err := OpenaiImageHandler(c, info, resp)
		require.Nil(t, err)
		require.Equal(t, 7, usage.TotalTokens)
		require.Contains(t, recorder.Body.String(), `"b64_json":"final"`)
		require.Contains(t, recorder.Body.String(), `"revised_prompt":"draw a cat"`)
		require.NotContains(t, recorder.Body.String(), `"url"`)
		require.NotContains(t, recorder.Body.String(), `"_provider_image_url"`)
		require.NotContains(t, recorder.Body.String(), `upstream.example`)
		require.NotContains(t, recorder.Body.String(), `provider.example`)
	}
}

func TestStripChannelImageURLsUsesExactFieldMatches(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{ForceImageB64JSONNoURL: true},
		},
	}
	body := []byte(`{"created":1710000000,"data":[{"b64_json":"one","url":"https://hidden.example/one.png","_provider_image_url":"https://hidden.example/provider.png","provider_image_url":"https://visible.example/provider.png","image_url":"https://visible.example/image.png"},{"_provider_image_url":"https://hidden.example/two.png","b64_json":"two"}],"usage":{"total_tokens":7}}`)

	stripped := stripChannelImageURLs(body, info)

	require.JSONEq(t, `{"created":1710000000,"data":[{"b64_json":"one","provider_image_url":"https://visible.example/provider.png","image_url":"https://visible.example/image.png"},{"b64_json":"two"}],"usage":{"total_tokens":7}}`, string(stripped))
	require.NotContains(t, string(stripped), `hidden.example`)
	require.Contains(t, string(stripped), `visible.example`)
}

func TestOpenaiImageHandlerKeepsOtherChannelImageURL(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"created":1710000000,"data":[{"b64_json":"final","url":"https://upstream.example/image.png"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`

	c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)

	usage, err := OpenaiImageHandler(c, info, resp)
	require.Nil(t, err)
	require.Equal(t, 7, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"url":"https://upstream.example/image.png"`)
}

func TestValidateChannelImageResponseURLs(t *testing.T) {
	tests := []struct {
		name     string
		settings dto.ChannelOtherSettings
		body     string
		wantErr  bool
	}{
		{
			name: "hide switch ignores prefix and strips later",
			settings: dto.ChannelOtherSettings{
				ForceImageB64JSONNoURL: true,
				ImageResponseURLPrefix: "https://trusted.example/",
			},
			body: `{"data":[{"url":"https://changed.example/image.png"}]}`,
		},
		{
			name:     "empty prefix allows any URL",
			settings: dto.ChannelOtherSettings{},
			body:     `{"data":[{"url":"https://changed.example/image.png"}]}`,
		},
		{
			name: "matching fields are allowed",
			settings: dto.ChannelOtherSettings{
				ImageResponseURLPrefix: "  https://trusted.example/images/  ",
			},
			body: `{"data":[{"url":"https://trusted.example/images/one.png","_provider_image_url":"https://trusted.example/images/two.png"}]}`,
		},
		{
			name: "escaped matching URL is allowed",
			settings: dto.ChannelOtherSettings{
				ImageResponseURLPrefix: "https://trusted.example/images/",
			},
			body: `{"data":[{"url":"https:\/\/trusted.example\/images\/one.png"}]}`,
		},
		{
			name: "url mismatch is rejected",
			settings: dto.ChannelOtherSettings{
				ImageResponseURLPrefix: "https://trusted.example/images/",
			},
			body:    `{"data":[{"url":"https://changed.example/image.png"}]}`,
			wantErr: true,
		},
		{
			name: "provider URL mismatch is rejected",
			settings: dto.ChannelOtherSettings{
				ImageResponseURLPrefix: "https://trusted.example/images/",
			},
			body:    `{"data":[{"url":"https://trusted.example/images/one.png"},{"_provider_image_url":"https://changed.example/two.png"}]}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{ChannelOtherSettings: tt.settings},
			}

			err := validateChannelImageResponseURLs([]byte(tt.body), info)
			if !tt.wantErr {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, http.StatusBadGateway, err.StatusCode)
			require.Equal(t, "openai error.", err.Error())
			require.Equal(t, "openai error.", err.ToOpenAIError().Message)
		})
	}
}

func TestOpenaiImageHandlersRejectUntrustedResponseURL(t *testing.T) {
	settings := dto.ChannelOtherSettings{ImageResponseURLPrefix: "https://trusted.example/images/"}
	jsonBody := `{"created":1710000000,"data":[{"url":"https://changed.example/image.png"}],"usage":{"total_tokens":7}}`

	for _, relayMode := range []int{relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits} {
		t.Run(fmt.Sprintf("json relay mode %d", relayMode), func(t *testing.T) {
			c, recorder, resp, info := newImageTestContext(t, jsonBody, "application/json", false)
			info.RelayMode = relayMode
			info.ChannelMeta.ChannelOtherSettings = settings

			usage, err := OpenaiImageHandler(c, info, resp)

			require.Nil(t, usage)
			require.NotNil(t, err)
			require.Equal(t, http.StatusBadGateway, err.StatusCode)
			require.Equal(t, "openai error.", err.Error())
			require.Empty(t, recorder.Body.String())
		})
	}

	t.Run("JSON converted to stream", func(t *testing.T) {
		c, recorder, resp, info := newImageTestContext(t, jsonBody, "application/json", true)
		info.ChannelMeta.ChannelOtherSettings = settings

		usage, err := OpenaiImageStreamHandler(c, info, resp)

		require.Nil(t, usage)
		require.NotNil(t, err)
		require.Equal(t, http.StatusBadGateway, err.StatusCode)
		require.Equal(t, "openai error.", err.Error())
		require.Empty(t, recorder.Body.String())
	})

	t.Run("native SSE", func(t *testing.T) {
		streamBody := "data: {\"type\":\"image_generation.completed\",\"url\":\"https://changed.example/image.png\"}\n\ndata: [DONE]\n\n"
		c, recorder, resp, info := newImageTestContext(t, streamBody, "text/event-stream", true)
		info.ChannelMeta.ChannelOtherSettings = settings

		usage, err := OpenaiImageStreamHandler(c, info, resp)

		require.Nil(t, usage)
		require.NotNil(t, err)
		require.Equal(t, http.StatusBadGateway, err.StatusCode)
		require.Equal(t, "openai error.", err.Error())
		require.Empty(t, recorder.Body.String())
	})
}

func TestOpenaiImageHandlerCopiesRequestedQualityToTopLevel(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	longImage := strings.Repeat("a", 4096)
	responseOverride := map[string]interface{}{
		"operations": []interface{}{
			map[string]interface{}{
				"phase": "response",
				"path":  "quality",
				"mode":  "set_from_request",
				"from":  "quality",
				"conditions": []interface{}{
					map[string]interface{}{
						"path":  "model",
						"mode":  "full",
						"value": "gpt-image-2",
					},
				},
				"logic": "AND",
			},
		},
	}

	for _, relayMode := range []int{relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits} {
		t.Run(fmt.Sprintf("relay mode %d", relayMode), func(t *testing.T) {
			body := `{"created":1785384134,"data":[{"b64_json":"` + longImage + `"}],"output_format":"png","quality":"high","size":"1254x1254","usage":{"total_tokens":279}}`
			c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
			info.RelayMode = relayMode
			info.ChannelMeta.ParamOverride = responseOverride
			info.ChannelMeta.UpstreamModelName = "gpt-image-2"
			info.Request = &dto.ImageRequest{Model: "client-alias", Quality: "low"}

			usage, apiErr := OpenaiImageHandler(c, info, resp)
			require.Nil(t, apiErr)
			require.Equal(t, 279, usage.TotalTokens)
			require.JSONEq(t, `{"created":1785384134,"data":[{"b64_json":"`+longImage+`"}],"output_format":"png","quality":"low","size":"1254x1254","usage":{"total_tokens":279}}`, recorder.Body.String())
		})
	}
}

func TestOpenaiImageHandlerLeavesQualityWithoutResponseOperation(t *testing.T) {
	body := `{"data":[],"quality":"high"}`
	c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
	info.ChannelMeta.UpstreamModelName = "gpt-image-2"
	info.Request = &dto.ImageRequest{Model: "gpt-image-2", Quality: "low"}

	_, apiErr := OpenaiImageHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.JSONEq(t, body, recorder.Body.String())
}

func TestOpenaiImageHandlerNormalizesTopLevelFieldsWithoutChangingData(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	tests := []struct {
		name         string
		body         string
		request      *dto.ImageRequest
		wantCreated  int64
		wantFormat   string
		wantQuality  string
		wantSize     string
		createdByNow bool
	}{
		{
			name:        "upstream values take precedence",
			body:        `{"created":1787414536,"data":[{"url":"https://example.com/image.png","provider_metadata":{"seed":7},"b64_json":"YWJj"}],"output_format":"png","quality":"medium","size":"3840x2160"}`,
			request:     &dto.ImageRequest{OutputFormat: json.RawMessage(`"webp"`), Quality: "high", Size: "1536x1024"},
			wantCreated: 1787414536,
			wantFormat:  "png",
			wantQuality: "medium",
			wantSize:    "3840x2160",
		},
		{
			name:        "request values fill missing fields",
			body:        `{"created":1787414536,"data":[{"custom":"kept","b64_json":"YWJj"}]}`,
			request:     &dto.ImageRequest{OutputFormat: json.RawMessage(`"webp"`), Quality: "high", Size: "1536x1024"},
			wantCreated: 1787414536,
			wantFormat:  "webp",
			wantQuality: "high",
			wantSize:    "1536x1024",
		},
		{
			name:         "defaults fill missing request values",
			body:         `{"data":[{"custom":"kept","b64_json":"YWJj"}]}`,
			request:      &dto.ImageRequest{},
			wantFormat:   "png",
			wantQuality:  "medium",
			wantSize:     "auto",
			createdByNow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, relayMode := range []int{relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits} {
				t.Run(fmt.Sprintf("relay mode %d", relayMode), func(t *testing.T) {
					c, recorder, resp, info := newImageTestContext(t, tt.body, "application/json", false)
					info.RelayMode = relayMode
					info.Request = tt.request
					info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{NormalizeOpenAIImageResponse: true}
					before := time.Now().Unix()

					_, apiErr := OpenaiImageHandler(c, info, resp)

					require.Nil(t, apiErr)
					output := recorder.Body.Bytes()
					require.Equal(t, gjson.Get(tt.body, "data").Raw, gjson.GetBytes(output, "data").Raw)
					require.Equal(t, tt.wantFormat, gjson.GetBytes(output, "output_format").String())
					require.Equal(t, tt.wantQuality, gjson.GetBytes(output, "quality").String())
					require.Equal(t, tt.wantSize, gjson.GetBytes(output, "size").String())
					if tt.createdByNow {
						require.GreaterOrEqual(t, gjson.GetBytes(output, "created").Int(), before)
						require.LessOrEqual(t, gjson.GetBytes(output, "created").Int(), time.Now().Unix())
					} else {
						require.Equal(t, tt.wantCreated, gjson.GetBytes(output, "created").Int())
					}
				})
			}
		})
	}
}

func TestOpenaiImageHandlerNormalizesQuality(t *testing.T) {
	tests := []struct {
		name           string
		upstreamField  string
		requestQuality string
		wantQualityRaw string
	}{
		{name: "preserves upstream string", upstreamField: `,"quality":"upstream-quality"`, requestQuality: "high", wantQualityRaw: `"upstream-quality"`},
		{name: "preserves upstream empty string", upstreamField: `,"quality":""`, requestQuality: "high", wantQualityRaw: `""`},
		{name: "preserves upstream null", upstreamField: `,"quality":null`, requestQuality: "high", wantQualityRaw: `null`},
		{name: "uses request value when upstream field is missing", requestQuality: "custom", wantQualityRaw: `"custom"`},
		{name: "uses medium when both values are missing", wantQualityRaw: `"medium"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, relayMode := range []int{relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits} {
				t.Run(fmt.Sprintf("relay mode %d", relayMode), func(t *testing.T) {
					body := `{"data":[{"url":"https://example.com/image.png"}]` + tt.upstreamField + `}`
					c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
					info.RelayMode = relayMode
					info.Request = &dto.ImageRequest{Quality: tt.requestQuality}
					info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{NormalizeOpenAIImageResponse: true}

					_, apiErr := OpenaiImageHandler(c, info, resp)

					require.Nil(t, apiErr)
					require.Equal(t, tt.wantQualityRaw, gjson.GetBytes(recorder.Body.Bytes(), "quality").Raw)
				})
			}
		})
	}
}

func TestOpenaiImageHandlerNormalizesUsageLeaves(t *testing.T) {
	tests := []struct {
		name      string
		usage     string
		wantUsage string
	}{
		{
			name:      "native image usage fills missing details",
			usage:     `{"input_tokens":60,"output_tokens":1756,"total_tokens":1816,"input_tokens_details":{"text_tokens":60}}`,
			wantUsage: `{"input_tokens":60,"input_tokens_details":{"image_tokens":0,"text_tokens":60},"output_tokens":1756,"total_tokens":1816,"output_tokens_details":{"image_tokens":0,"text_tokens":0}}`,
		},
		{
			name:      "legacy aliases map leaf by leaf",
			usage:     `{"prompt_tokens":60,"completion_tokens":1756,"total_tokens":1816,"prompt_tokens_details":{"text_tokens":60,"image_tokens":4},"completion_tokens_details":{"text_tokens":2,"image_tokens":1754},"claude_cache_creation_5_m_tokens":9}`,
			wantUsage: `{"input_tokens":60,"input_tokens_details":{"image_tokens":4,"text_tokens":60},"output_tokens":1756,"total_tokens":1816,"output_tokens_details":{"image_tokens":1754,"text_tokens":2}}`,
		},
		{
			name:      "explicit primary zero wins per leaf",
			usage:     `{"input_tokens":0,"prompt_tokens":99,"input_tokens_details":{"text_tokens":0},"prompt_tokens_details":{"text_tokens":88,"image_tokens":7}}`,
			wantUsage: `{"input_tokens":0,"input_tokens_details":{"image_tokens":7,"text_tokens":0},"output_tokens":0,"total_tokens":0,"output_tokens_details":{"image_tokens":0,"text_tokens":0}}`,
		},
		{
			name:      "invalid primary values use valid aliases or zero",
			usage:     `{"input_tokens":-1,"prompt_tokens":12,"output_tokens":1.5,"completion_tokens":13,"total_tokens":9223372036854775808,"input_tokens_details":{"image_tokens":"3","text_tokens":{}},"prompt_tokens_details":{"image_tokens":4,"text_tokens":5},"output_tokens_details":{"image_tokens":-2,"text_tokens":"7"},"completion_tokens_details":{"image_tokens":6,"text_tokens":7}}`,
			wantUsage: `{"input_tokens":12,"input_tokens_details":{"image_tokens":4,"text_tokens":5},"output_tokens":13,"total_tokens":0,"output_tokens_details":{"image_tokens":6,"text_tokens":7}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"data":[],"usage":` + tt.usage + `}`
			c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
			info.Request = &dto.ImageRequest{}
			info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{NormalizeOpenAIImageResponse: true}

			_, apiErr := OpenaiImageHandler(c, info, resp)

			require.Nil(t, apiErr)
			require.JSONEq(t, tt.wantUsage, gjson.Get(recorder.Body.String(), "usage").Raw)
		})
	}
}

func TestOpenaiImageHandlerDoesNotNormalizeJSONStreamFallback(t *testing.T) {
	body := `{"created":1710000000,"data":[{"b64_json":"first"}]}`
	c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)
	info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{NormalizeOpenAIImageResponse: true}
	info.Request = &dto.ImageRequest{}

	_, apiErr := OpenaiImageStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotContains(t, recorder.Body.String(), `"output_format"`)
	require.NotContains(t, recorder.Body.String(), `"quality"`)
	require.NotContains(t, recorder.Body.String(), `"size"`)
}

func TestOpenaiImageStreamHandlerStripsChannelJSONFallbackURL(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"created":1710000000,"data":[{"b64_json":"final","url":"https://upstream.example/image.png","revised_prompt":"draw a cat"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`

	c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)
	info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{ForceImageB64JSONNoURL: true}

	usage, err := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.Equal(t, 7, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"b64_json":"final"`)
	require.NotContains(t, recorder.Body.String(), `"url"`)
	require.NotContains(t, recorder.Body.String(), `upstream.example`)
}

// TestOpenaiImageHandlersReturnJSONError covers JSON error responses for both
// entry points: the non-streaming handler and the stream handler's non-SSE
// fallback. Neither must leak the error body to the client.
func TestOpenaiImageHandlersReturnJSONError(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"error":{"message":"content moderation failed","type":"upstream_error","code":"content_moderation_failed","status":502}}`

	t.Run("non-streaming handler", func(t *testing.T) {
		c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
		info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{NormalizeOpenAIImageResponse: true}

		usage, err := OpenaiImageHandler(c, info, resp)
		require.Nil(t, usage)
		require.NotNil(t, err)
		require.Equal(t, http.StatusOK, err.StatusCode)
		oaiError := err.ToOpenAIError()
		require.Equal(t, "content moderation failed", oaiError.Message)
		require.Equal(t, "upstream_error", oaiError.Type)
		require.Equal(t, "content_moderation_failed", oaiError.Code)
		require.Empty(t, recorder.Body.String())
	})

	t.Run("non-2xx response without OpenAI error is not normalized", func(t *testing.T) {
		plainBody := `{"message":"upstream unavailable"}`
		c, recorder, resp, info := newImageTestContext(t, plainBody, "application/json", false)
		resp.StatusCode = http.StatusBadGateway
		info.ChannelMeta.ChannelOtherSettings = dto.ChannelOtherSettings{NormalizeOpenAIImageResponse: true}

		_, err := OpenaiImageHandler(c, info, resp)

		require.Nil(t, err)
		require.JSONEq(t, plainBody, recorder.Body.String())
	})

	t.Run("stream handler JSON fallback", func(t *testing.T) {
		c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)

		usage, err := OpenaiImageStreamHandler(c, info, resp)
		require.Nil(t, usage)
		require.NotNil(t, err)
		require.Equal(t, http.StatusOK, err.StatusCode)
		require.Equal(t, "content moderation failed", err.ToOpenAIError().Message)
		require.Empty(t, recorder.Body.String())
	})

	t.Run("stream handler non-2xx stays JSON error", func(t *testing.T) {
		c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)
		resp.StatusCode = http.StatusBadGateway

		usage, err := OpenaiImageStreamHandler(c, info, resp)
		require.Nil(t, usage)
		require.NotNil(t, err)
		require.Equal(t, http.StatusBadGateway, err.StatusCode)
		require.Equal(t, "content moderation failed", err.ToOpenAIError().Message)
		require.Empty(t, recorder.Body.String())
		require.NotContains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	})
}

// TestOpenaiImageStreamHandlerRecordsUpstreamErrorEvent verifies that an error
// event inside the SSE stream is recorded as a soft error while the payload is
// still forwarded to the client.
func TestOpenaiImageStreamHandlerRecordsUpstreamErrorEvent(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`event: image_generation.partial_image`,
		`data: {"type":"image_generation.partial_image","b64_json":"partial"}`,
		``,
		`event: error`,
		`data: {"type":"upstream_error","error":{"message":"stream error: stream ID 77; INTERNAL_ERROR; received from peer"}}`,
		``,
	}, "\n")

	c, recorder, resp, info := newImageTestContext(t, body, "text/event-stream", true)

	usage, err := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
	require.True(t, info.StreamStatus.HasErrors())
	require.Equal(t, 1, info.StreamStatus.TotalErrorCount())
	require.Contains(t, info.StreamStatus.Errors[0].Message, "INTERNAL_ERROR")
	// The scanner strips the upstream "event: error" line; the event name is
	// rebuilt from the JSON "type" field (upstream_error). The error message
	// is still forwarded in the data: payload (stream ID 77).
	require.Contains(t, recorder.Body.String(), `event: upstream_error`)
	require.Contains(t, recorder.Body.String(), `stream ID 77`)
}

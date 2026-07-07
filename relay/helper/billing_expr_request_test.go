package helper

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResolveIncomingBillingExprRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")

	body := []byte(`{"service_tier":"fast"}`)
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
	ctx.Set(common.KeyRequestBody, body)

	info := &relaycommon.RelayInfo{
		RequestHeaders: map[string]string{"Content-Type": "application/json"},
	}

	input, err := ResolveIncomingBillingExprRequestInput(ctx, info)
	require.NoError(t, err)
	require.Equal(t, body, input.Body)
	require.Equal(t, "application/json", input.Headers["Content-Type"])
}

func TestBuildBillingExprRequestInputFromRequest(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:  "gemini-3.1-pro-preview",
		Stream: lo.ToPtr(true),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
		MaxTokens: lo.ToPtr(uint(3000)),
	}

	input, err := BuildBillingExprRequestInputFromRequest(request, map[string]string{
		"Content-Type": "application/json",
		"X-Test":       "1",
	})
	require.NoError(t, err)
	require.Equal(t, "application/json", input.Headers["Content-Type"])
	require.Equal(t, "1", input.Headers["X-Test"])
	require.True(t, gjson.GetBytes(input.Body, "stream").Bool())
	require.Equal(t, "user", gjson.GetBytes(input.Body, "messages.0.role").String())
	require.Equal(t, float64(3000), gjson.GetBytes(input.Body, "max_tokens").Float())
}

func TestBuildBillingExprRequestInputFromImageGenerationRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader([]byte(`{
		"model": "gpt-image-2",
		"prompt": "draw a cat",
		"size": "1536x864",
		"n": 2
	}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	body, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	c.Set(common.KeyRequestBody, body)

	request, err := GetAndValidateRequest(c, types.RelayFormatOpenAIImage)
	require.NoError(t, err)
	input, err := BuildBillingExprRequestInputFromRequest(request, map[string]string{
		"Content-Type": "application/json",
	})
	require.NoError(t, err)
	require.Equal(t, "1536x864", gjson.GetBytes(input.Body, "size").String())
	require.Equal(t, float64(2), gjson.GetBytes(input.Body, "n").Float())
}

func TestBuildBillingExprRequestInputFromImageEditMultipartRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit this image"))
	require.NoError(t, writer.WriteField("size", "2560x1600"))
	require.NoError(t, writer.WriteField("n", "3"))
	part, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	request, err := GetAndValidateRequest(c, types.RelayFormatOpenAIImage)
	require.NoError(t, err)
	input, err := BuildBillingExprRequestInputFromRequest(request, map[string]string{
		"Content-Type": c.Request.Header.Get("Content-Type"),
	})
	require.NoError(t, err)
	require.Equal(t, "2560x1600", gjson.GetBytes(input.Body, "size").String())
	require.Equal(t, float64(3), gjson.GetBytes(input.Body, "n").Float())
}

package oaichat

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIChatRequestToGeminiImageGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request := dto.GeneralOpenAIRequest{
		Model: "gemini-3.1-flash-image-preview",
		Messages: []dto.Message{
			{Role: "user", Content: "Create a studio product photo"},
		},
		ExtraBody: []byte(`{"google":{"image_config":{"image_size":"4K","aspect_ratio":"16:9"}}}`),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: request.Model,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: request.Model,
		},
	}

	converted, err := OpenAIChatRequestToGeminiGenerateContent(context, request, info)
	require.NoError(t, err)
	assert.Equal(t, []string{"TEXT", "IMAGE"}, converted.GenerationConfig.ResponseModalities)
	require.Len(t, converted.Contents, 1)
	require.Len(t, converted.Contents[0].Parts, 1)
	assert.Equal(t, "Create a studio product photo", converted.Contents[0].Parts[0].Text)

	var imageConfig map[string]any
	require.NoError(t, common.Unmarshal(converted.GenerationConfig.ImageConfig, &imageConfig))
	assert.Equal(t, "4K", imageConfig["imageSize"])
	assert.Equal(t, "16:9", imageConfig["aspectRatio"])
}

package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyImagePromptParameterAppend(t *testing.T) {
	tests := []struct {
		name        string
		config      *dto.ImagePromptParameterAppendConfig
		model       string
		wantPrompt  string
		wantApplied bool
	}{
		{
			name:        "applies to matched model",
			config:      &dto.ImagePromptParameterAppendConfig{Enabled: true},
			model:       "gpt-image-2",
			wantPrompt:  "draw a cat\n\nOutput image requirements: size=2048x1152; quality=high.",
			wantApplied: true,
		},
		{
			name:       "skips unmatched model",
			config:     &dto.ImagePromptParameterAppendConfig{Enabled: true},
			model:      "gpt-image-1",
			wantPrompt: "draw a cat",
		},
		{
			name:       "skips disabled configuration",
			config:     &dto.ImagePromptParameterAppendConfig{},
			model:      "gpt-image-2",
			wantPrompt: "draw a cat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.ImageRequest{
				Model:   tt.model,
				Prompt:  "draw a cat",
				Size:    "2048x1152",
				Quality: "high",
			}
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{ImagePromptParameterAppend: tt.config},
				},
			}

			applied, err := applyImagePromptParameterAppend(info, request)
			require.NoError(t, err)
			assert.Equal(t, tt.wantApplied, applied)
			assert.Equal(t, tt.wantPrompt, request.Prompt)
		})
	}
}

func TestApplyImageRequestTemplateOverride(t *testing.T) {
	request := &dto.ImageRequest{
		Model:   "gpt-image-2",
		Prompt:  "draw a cat",
		Size:    "1254x1254",
		Quality: "low",
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []interface{}{
					map[string]interface{}{
						"phase": "request",
						"path":  "prompt",
						"mode":  "append_template",
						"value": "\n\nOutput image requirements: size=${body.size}; quality=${body.quality}.",
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
			},
		},
	}

	applied, err := applyImageRequestTemplateOverride(info, request)
	require.NoError(t, err)
	require.True(t, applied)
	assert.Equal(t, "draw a cat\n\nOutput image requirements: size=1254x1254; quality=low.", request.Prompt)
}

func TestCompositeImageRequestDisablesBodyPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	globalSettings := model_setting.GetGlobalSettings()
	originalGlobalPassthrough := globalSettings.PassThroughRequestEnabled
	t.Cleanup(func() {
		globalSettings.PassThroughRequestEnabled = originalGlobalPassthrough
	})

	globalSettings.PassThroughRequestEnabled = false
	assert.True(t, shouldPassThroughImageRequest(c, true, false))
	assert.False(t, shouldPassThroughImageRequest(c, true, true))

	globalSettings.PassThroughRequestEnabled = true
	assert.True(t, shouldPassThroughImageRequest(c, false, false))
	assert.False(t, shouldPassThroughImageRequest(c, false, true))

	common.SetContextKey(c, constant.ContextKeyCompositeDisableRequestBodyPassthrough, true)
	assert.False(t, shouldPassThroughImageRequest(c, true, false))
	assert.False(t, shouldPassThroughImageRequest(c, false, false))
}

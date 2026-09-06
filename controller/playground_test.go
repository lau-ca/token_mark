package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointSupportsPlaygroundCapability(t *testing.T) {
	tests := []struct {
		name       string
		config     dto.ModelEndpointConfig
		capability string
		want       bool
	}{
		{
			name:       "unconfigured image endpoint is rejected",
			capability: "image.generate",
			want:       false,
		},
		{
			name:       "legacy image endpoint does not infer editing",
			capability: "image.edit",
			want:       false,
		},
		{
			name: "explicit capabilities restrict the endpoint",
			config: dto.ModelEndpointConfig{Playground: &dto.ModelPlaygroundConfig{
				Capabilities: []string{"video.image_to_video"},
			}},
			capability: "video.text_to_video",
			want:       false,
		},
		{
			name: "explicit image to video capability is accepted",
			config: dto.ModelEndpointConfig{Playground: &dto.ModelPlaygroundConfig{
				Capabilities: []string{"video.image_to_video"},
			}},
			capability: "video.image_to_video",
			want:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, endpointSupportsPlaygroundCapability(test.config, test.capability))
		})
	}
}

func TestHasPlaygroundReference(t *testing.T) {
	assert.False(t, hasPlaygroundReference(map[string]any{}))
	assert.False(t, hasPlaygroundReference(map[string]any{"image": "  "}))
	assert.True(t, hasPlaygroundReference(map[string]any{"image": "https://example.com/reference.png"}))
	assert.True(t, hasPlaygroundReference(map[string]any{
		"messages": []any{
			map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "animate"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,a"}},
				},
			},
		},
	}))
}

func TestBuildPlaygroundEndpointConfigsCombinesMetadataRuntimeAndCapabilities(t *testing.T) {
	endpointTypes, endpoints, err := buildPlaygroundEndpointConfigs(
		`{"openai":{"path":"/custom/chat","method":"POST","future":"kept"}}`,
		`{"endpoints":{"image-generation":{"capabilities":["image.generate"]}}}`,
		[]constant.EndpointType{constant.EndpointTypeOpenAI},
	)

	require.NoError(t, err)
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeImageGeneration,
	}, endpointTypes)
	assert.Equal(t, "/custom/chat", endpoints[string(constant.EndpointTypeOpenAI)].Path)
	imageEndpoint := endpoints[string(constant.EndpointTypeImageGeneration)]
	require.NotNil(t, imageEndpoint.Playground)
	assert.Equal(t, []string{"image.generate"}, imageEndpoint.Playground.Capabilities)
	assert.NotEmpty(t, imageEndpoint.Path)
	assert.Equal(t, "POST", imageEndpoint.Method)
}

package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultModelEndpointConfig(t *testing.T) {
	config, ok := GetDefaultModelEndpointConfig(constant.EndpointTypeOpenAIVideo)
	require.True(t, ok)
	assert.Equal(t, "/v1/videos", config.Path)
	assert.Equal(t, "POST", config.Method)
}

func TestParseModelEndpointConfigs(t *testing.T) {
	t.Run("preserves legacy endpoint arrays", func(t *testing.T) {
		configs, err := ParseModelEndpointConfigs(`["openai","openai-response"]`)
		require.NoError(t, err)
		assert.Contains(t, configs, "openai")
		assert.Contains(t, configs, "openai-response")
	})

	t.Run("parses playground parameters", func(t *testing.T) {
		configs, err := ParseModelEndpointConfigs(`{
			"image-generation": {
				"path": "/v1/images/generations",
				"method": "POST",
				"playground": {
					"capabilities": ["image.generate"],
					"parameters": [{
						"key": "size",
						"type": "enum",
						"default": "1024x1024",
						"options": ["1024x1024", "2048x2048"]
					}]
				}
			}
		}`)
		require.NoError(t, err)
		config := configs["image-generation"]
		require.NotNil(t, config.Playground)
		assert.Equal(t, []string{"image.generate"}, config.Playground.Capabilities)
		assert.Equal(t, "size", config.Playground.Parameters[0].Key)
	})

	t.Run("rejects invalid numeric ranges", func(t *testing.T) {
		_, err := ParseModelEndpointConfigs(`{
			"openai-video": {
				"playground": {
					"parameters": [{"key":"duration","type":"number","min":15,"max":4}]
				}
			}
		}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "min greater than max")
	})

	t.Run("rejects defaults outside enum options", func(t *testing.T) {
		_, err := ParseModelEndpointConfigs(`{
			"image-generation": {
				"playground": {
					"parameters": [{"key":"size","type":"enum","default":"4K","options":["1K","2K"]}]
				}
			}
		}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid default")
	})

	t.Run("rejects unsafe request paths", func(t *testing.T) {
		_, err := ParseModelEndpointConfigs(`{
			"gemini": {
				"playground": {
					"parameters": [{"key":"size","type":"string","request_path":"__proto__.size"}]
				}
			}
		}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsafe segment")
	})

	t.Run("rejects boolean defaults encoded as strings", func(t *testing.T) {
		_, err := ParseModelEndpointConfigs(`{
			"openai-video": {
				"playground": {
					"parameters": [{"key":"enhance","type":"boolean","default":"false"}]
				}
			}
		}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a boolean")
	})

	t.Run("parses integration templates", func(t *testing.T) {
		configs, err := ParseModelEndpointConfigs(`{
			"openai-video": {
				"playground": {
					"capabilities": ["video.text_to_video"],
					"integration": {
						"overview": "Create then query the task.",
						"documentation_url": "https://ai-doc.apifox.cn/",
						"interfaces": [
							{"key":"create","title":"Create","method":"POST","path":"/v1/videos","curl_template":"curl '{{base_url}}/v1/videos'"},
							{"key":"query","title":"Query","method":"GET","path":"/v1/videos/{{task_id}}","curl_template":"curl '{{base_url}}/v1/videos/{{task_id}}'"}
						]
					}
				}
			}
		}`)
		require.NoError(t, err)
		require.NotNil(t, configs["openai-video"].Playground.Integration)
		assert.Len(t, configs["openai-video"].Playground.Integration.Interfaces, 2)
	})

	for _, test := range []struct {
		name        string
		integration string
		wantError   string
	}{
		{
			name: "rejects duplicate integration keys",
			integration: `{"interfaces":[
				{"key":"task","title":"Create","method":"POST","path":"/v1/videos","curl_template":"curl x"},
				{"key":"task","title":"Query","method":"GET","path":"/v1/videos/{{task_id}}","curl_template":"curl y"}
			]}`,
			wantError: "duplicate interface key",
		},
		{
			name: "rejects unsupported methods",
			integration: `{"interfaces":[
				{"key":"create","title":"Create","method":"OPTIONS","path":"/v1/videos","curl_template":"curl x"}
			]}`,
			wantError: "unsupported method",
		},
		{
			name: "rejects invalid paths",
			integration: `{"interfaces":[
				{"key":"create","title":"Create","method":"POST","path":"v1/videos","curl_template":"curl x"}
			]}`,
			wantError: "path must start with /",
		},
		{
			name: "rejects unknown template variables",
			integration: `{"interfaces":[
				{"key":"create","title":"Create","method":"POST","path":"/v1/videos","curl_template":"curl '{{secret}}'"}
			]}`,
			wantError: "unsupported template variable",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseModelEndpointConfigs(`{"openai-video":{"playground":{"integration":` + test.integration + `}}}`)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestValidatePlaygroundParameterValues(t *testing.T) {
	minDuration := float64(1)
	maxDuration := float64(15)
	config := &ModelPlaygroundConfig{Parameters: []PlaygroundParameter{
		{Key: "resolution", Type: PlaygroundParameterEnum, Options: []any{"480p", "720p"}},
		{Key: "duration", Type: PlaygroundParameterNumber, Min: &minDuration, Max: &maxDuration},
	}}

	require.NoError(t, ValidatePlaygroundParameterValues(config, map[string]any{
		"resolution": "720p",
		"duration":   float64(5),
	}))
	err := ValidatePlaygroundParameterValues(config, map[string]any{
		"resolution": "1080p",
		"duration":   float64(5),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported value")

	nestedConfig := &ModelPlaygroundConfig{Parameters: []PlaygroundParameter{
		{
			Key:         "image_size",
			Type:        PlaygroundParameterEnum,
			RequestPath: "extra_body.google.image_config.image_size",
			Options:     []any{"1K", "2K"},
		},
	}}
	require.NoError(t, ValidatePlaygroundParameterValues(nestedConfig, map[string]any{
		"extra_body": map[string]any{
			"google": map[string]any{
				"image_config": map[string]any{"image_size": "2K"},
			},
		},
	}))
	err = ValidatePlaygroundParameterValues(nestedConfig, map[string]any{
		"extra_body": map[string]any{
			"google": map[string]any{
				"image_config": map[string]any{"image_size": "4K"},
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported value")
}

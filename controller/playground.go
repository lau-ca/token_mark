package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

const playgroundCapabilityHeader = "X-Playground-Capability"

func readPlaygroundValues(c *gin.Context) (map[string]any, string, error) {
	values := make(map[string]any)
	modelName := ""
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, "", err
		}
		for key, entries := range form.Value {
			if len(entries) > 0 {
				values[key] = entries[0]
			}
		}
		for key, files := range form.File {
			if len(files) > 0 {
				values[key] = files[0].Filename
			}
		}
		modelName, _ = values["model"].(string)
	} else {
		if err := common.UnmarshalBodyReusable(c, &values); err != nil {
			return nil, "", err
		}
		modelName, _ = values["model"].(string)
	}
	return values, modelName, nil
}

func endpointSupportsPlaygroundCapability(config dto.ModelEndpointConfig, capability string) bool {
	if config.Playground == nil {
		return false
	}
	for _, configured := range config.Playground.Capabilities {
		if configured == capability {
			return true
		}
	}
	return false
}

func validatePlaygroundRequest(c *gin.Context, endpointName string, capability string) (map[string]any, error) {
	values, modelName, err := readPlaygroundValues(c)
	if err != nil {
		return nil, err
	}
	if modelName == "" {
		return nil, errors.New("model is required")
	}

	capabilities, err := model.GetModelCapabilitiesByNames([]string{modelName})
	if err != nil {
		return nil, err
	}
	endpoints := map[string]dto.ModelEndpointConfig{}
	if item := capabilities[modelName]; item != nil {
		config, parseErr := dto.ParseModelCapabilityConfig(item.Config)
		endpoints = config.EndpointConfigs()
		err = parseErr
		if err != nil {
			return nil, err
		}
	}
	if endpointName == "" {
		for _, endpoint := range endpoints {
			if endpointSupportsPlaygroundCapability(endpoint, capability) {
				if err := dto.ValidatePlaygroundParameterValues(endpoint.Playground, values); err != nil {
					return nil, err
				}
				return values, nil
			}
		}
		return nil, fmt.Errorf("model %s does not support %s", modelName, capability)
	}

	endpoint, ok := endpoints[endpointName]
	if !ok {
		return nil, fmt.Errorf("model %s does not support endpoint %s", modelName, endpointName)
	}
	if !endpointSupportsPlaygroundCapability(endpoint, capability) {
		return nil, fmt.Errorf("model %s does not support %s", modelName, capability)
	}
	if err := dto.ValidatePlaygroundParameterValues(endpoint.Playground, values); err != nil {
		return nil, err
	}
	return values, nil
}

func hasPlaygroundReference(values map[string]any) bool {
	for _, key := range []string{"image", "input_reference", "reference_image", "referenceImages"} {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return true
			}
		case []any:
			if len(typed) > 0 {
				return true
			}
		default:
			return true
		}
	}

	messages, _ := values["messages"].([]any)
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		parts, _ := message["content"].([]any)
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]any)
			if part["type"] == "image_url" {
				return true
			}
		}
	}
	return false
}

func setupPlaygroundContext(c *gin.Context, relayFormat types.RelayFormat) *types.NewAPIError {
	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		return types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	userId := c.GetInt("id")
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)
	return nil
}

func relayPlayground(c *gin.Context, relayFormat types.RelayFormat) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	newAPIError = setupPlaygroundContext(c, relayFormat)
	if newAPIError == nil {
		Relay(c, relayFormat)
	}
}

func Playground(c *gin.Context) {
	capability := strings.TrimSpace(c.GetHeader(playgroundCapabilityHeader))
	endpointName := ""
	if capability == "" {
		values, _, err := readPlaygroundValues(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
			return
		}
		if _, hasImageConfig := dto.GetPlaygroundParameterValue(values, "extra_body.google.image_config"); hasImageConfig {
			capability = "image.generate"
			endpointName = "gemini"
			if hasPlaygroundReference(values) {
				capability = "image.edit"
			}
		} else {
			capability = "chat"
		}
	}
	if capability == "image.generate" || capability == "image.edit" {
		endpointName = "gemini"
	} else if capability != "chat" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "unsupported playground capability", "type": "invalid_request_error"}})
		return
	}
	if _, err := validatePlaygroundRequest(c, endpointName, capability); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
		return
	}
	relayPlayground(c, types.RelayFormatOpenAI)
}

func PlaygroundImage(c *gin.Context) {
	capability := "image.generate"
	if strings.HasSuffix(c.Request.URL.Path, "/edits") {
		capability = "image.edit"
	}
	if _, err := validatePlaygroundRequest(c, "image-generation", capability); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
		return
	}
	relayPlayground(c, types.RelayFormatOpenAIImage)
}

func PlaygroundTask(c *gin.Context) {
	values, _, err := readPlaygroundValues(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
		return
	}
	capability := "video.text_to_video"
	if hasPlaygroundReference(values) {
		capability = "video.image_to_video"
	}
	if _, err := validatePlaygroundRequest(c, "openai-video", capability); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
		return
	}
	if newAPIError := setupPlaygroundContext(c, types.RelayFormatTask); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}
	RelayTask(c)
}

func PlaygroundTaskFetch(c *gin.Context) {
	if newAPIError := setupPlaygroundContext(c, types.RelayFormatTask); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}
	RelayTaskFetch(c)
}

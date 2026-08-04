package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const qianfanVideoResourcePath = "/beta/video/generations/qianfan-video"

type qianfanResourceOperation string

const (
	qianfanCreateElement qianfanResourceOperation = "create_element"
	qianfanListElements  qianfanResourceOperation = "list_elements"
	qianfanCreateVoice   qianfanResourceOperation = "create_voice"
	qianfanListVoices    qianfanResourceOperation = "list_voices"
	qianfanPresetVoices  qianfanResourceOperation = "preset_voices"
	qianfanGetVoiceTask  qianfanResourceOperation = "get_voice_task"
	qianfanDeleteVoice   qianfanResourceOperation = "delete_voice"
)

func CreateQianfanElement(c *gin.Context) {
	proxyQianfanResource(c, qianfanCreateElement)
}

func ListQianfanElements(c *gin.Context) {
	proxyQianfanResource(c, qianfanListElements)
}

func CreateQianfanVoice(c *gin.Context) {
	proxyQianfanResource(c, qianfanCreateVoice)
}

func ListQianfanVoices(c *gin.Context) {
	proxyQianfanResource(c, qianfanListVoices)
}

func ListQianfanPresetVoices(c *gin.Context) {
	proxyQianfanResource(c, qianfanPresetVoices)
}

func GetQianfanVoiceTask(c *gin.Context) {
	proxyQianfanResource(c, qianfanGetVoiceTask)
}

func DeleteQianfanVoice(c *gin.Context) {
	proxyQianfanResource(c, qianfanDeleteVoice)
}

func proxyQianfanResource(c *gin.Context, operation qianfanResourceOperation) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid channel id"})
		return
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if channel.Type != constant.ChannelTypeBaiduV2 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "channel must be Baidu V2"})
		return
	}
	key, _, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		c.JSON(keyErr.StatusCode, gin.H{"success": false, "message": keyErr.Error()})
		return
	}
	request, err := buildQianfanResourceRequest(c, channel, key, operation)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	client, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	response, err := client.Do(request)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(response.StatusCode, contentType, responseBody)
}

func buildQianfanResourceRequest(c *gin.Context, channel *model.Channel, key string, operation qianfanResourceOperation) (*http.Request, error) {
	if channel == nil || channel.Type != constant.ChannelTypeBaiduV2 {
		return nil, fmt.Errorf("channel must be Baidu V2")
	}
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(constant.ChannelBaseURLs[constant.ChannelTypeBaiduV2], "/")
	}
	requestURL, err := url.Parse(baseURL + qianfanVideoResourcePath)
	if err != nil {
		return nil, err
	}
	method := http.MethodGet
	var body io.Reader

	switch operation {
	case qianfanCreateElement, qianfanCreateVoice:
		method = http.MethodPost
		var payload map[string]any
		if err := common.DecodeJson(c.Request.Body, &payload); err != nil {
			return nil, fmt.Errorf("invalid request body: %w", err)
		}
		if _, ok := payload["model_parameters"].(map[string]any); !ok {
			return nil, fmt.Errorf("model_parameters field is required")
		}
		if operation == qianfanCreateElement {
			payload["model"] = "Custom-Elements"
			payload["type"] = "omni-video"
		} else {
			payload["model"] = "Custom-Voices"
			payload["type"] = "custom-voices"
		}
		data, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	case qianfanListElements:
		query := requestURL.Query()
		query.Set("model", "Custom-Elements")
		pageNum, err := boundedPositiveQuery(c.Query("pageNum"), 1, 100000, "pageNum")
		if err != nil {
			return nil, err
		}
		pageSize, err := boundedPositiveQuery(c.Query("pageSize"), 20, 100, "pageSize")
		if err != nil {
			return nil, err
		}
		query.Set("pageNum", strconv.Itoa(pageNum))
		query.Set("pageSize", strconv.Itoa(pageSize))
		requestURL.RawQuery = query.Encode()
	case qianfanListVoices:
		requestURL.Path += "/list"
		query := requestURL.Query()
		query.Set("model", "Custom-Voices")
		requestURL.RawQuery = query.Encode()
	case qianfanPresetVoices:
		query := requestURL.Query()
		query.Set("model", "Presets-Voices")
		requestURL.RawQuery = query.Encode()
	case qianfanGetVoiceTask:
		taskID := strings.TrimSpace(c.Param("task_id"))
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}
		query := requestURL.Query()
		query.Set("model", "Custom-Voices")
		query.Set("task_id", taskID)
		requestURL.RawQuery = query.Encode()
	case qianfanDeleteVoice:
		voiceID := strings.TrimSpace(c.Param("voice_id"))
		if voiceID == "" {
			return nil, fmt.Errorf("voice_id is required")
		}
		method = http.MethodDelete
		data, err := common.Marshal(map[string]string{"voice_id": voiceID})
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	default:
		return nil, fmt.Errorf("unsupported qianfan resource operation")
	}

	request, err := http.NewRequestWithContext(c.Request.Context(), method, requestURL.String(), body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func boundedPositiveQuery(raw string, fallback, maximum int, field string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || value > maximum {
		return 0, fmt.Errorf("%s must be between 1 and %d", field, maximum)
	}
	return value, nil
}

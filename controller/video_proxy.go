package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// videoProxyError returns a standardized OpenAI-style error response.
func videoProxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func VideoProxy(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	userID := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}

	if task.Status != model.TaskStatusSuccess {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to get channel for task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	if channel.Type == constant.ChannelTypeSeedance {
		proxySeedanceVideo(c, task, channel)
		return
	}
	replaceVideoURLsWithProxy := false
	if constant.SupportsVideoURLProxyReplacement(channel.Type) {
		channelOtherSettings, settingsErr := channel.ParseOtherSettings()
		if settingsErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse channel settings for task %s: %s", taskID, settingsErr.Error()))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
			return
		}
		replaceVideoURLsWithProxy = channelOtherSettings.ReplaceVideoURLsWithProxy
	}
	if replaceVideoURLsWithProxy {
		c.Writer.Header().Set("Cache-Control", "private, no-store")
	}
	forwardRange := replaceVideoURLsWithProxy || channel.Type == constant.ChannelTypeBaiduV2
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	usesChannelContentURL := (channel.Type == constant.ChannelTypeOpenAI ||
		channel.Type == constant.ChannelTypeSora) &&
		!(replaceVideoURLsWithProxy && strings.HasPrefix(strings.TrimSpace(task.GetResultURL()), "data:"))

	var videoURL string
	skipFetchURLValidation := false
	proxy := channel.GetSetting().Proxy
	client := service.GetSSRFProtectedHTTPClient()
	if usesChannelContentURL {
		client = service.GetHttpClient()
	}
	if proxy != "" {
		// 渠道代理路径的连接由代理侧建立，无法做拨号时逐 IP 校验，
		// 因此后面对 videoURL 保留请求前的一次性 SSRF 校验。
		client, err = service.GetHttpClientWithProxy(proxy)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create proxy client for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy client")
			return
		}
	}
	if replaceVideoURLsWithProxy {
		privateClient := *client
		privateClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
		client = &privateClient
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create request: %s", err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}
	if forwardRange {
		req.Header.Set("Accept-Encoding", "identity")
		if value := c.GetHeader("Range"); value != "" {
			req.Header.Set("Range", value)
		}
		if value := c.GetHeader("If-Range"); value != "" {
			req.Header.Set("If-Range", value)
		}
	}

	switch channel.Type {
	case constant.ChannelTypeGemini:
		apiKey := task.PrivateData.Key
		if apiKey == "" {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Missing stored API key for Gemini task %s", taskID))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "API key not stored for task")
			return
		}
		videoURL, err = getGeminiVideoURL(channel, task, apiKey)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Gemini video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Gemini video URL")
			return
		}
		req.Header.Set("x-goog-api-key", apiKey)
	case constant.ChannelTypeVertexAi:
		videoURL, err = getVertexVideoURL(channel, task)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Vertex video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Vertex video URL")
			return
		}
	case constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
		if !usesChannelContentURL {
			videoURL = task.GetResultURL()
		} else {
			videoURL = fmt.Sprintf("%s/v1/videos/%s/content", baseURL, task.GetUpstreamTaskID())
			req.Header.Set("Authorization", "Bearer "+channel.Key)
			skipFetchURLValidation = true
		}
	default:
		// Video URL is stored in PrivateData.ResultURL (fallback to FailReason for old data)
		videoURL = task.GetResultURL()
	}

	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL is empty for task %s", taskID))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}

	if strings.HasPrefix(videoURL, "data:") {
		if replaceVideoURLsWithProxy {
			err = writePrivateVideoDataURL(c, videoURL)
		} else {
			err = writeVideoDataURL(c, videoURL)
		}
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to decode video data URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		}
		return
	}

	if !skipFetchURLValidation {
		var validateErr error
		if proxy == "" {
			validateErr = service.ValidateSSRFProtectedFetchURL(videoURL)
		} else {
			fetchSetting := system_setting.GetFetchSetting()
			validateErr = common.ValidateURLWithFetchSetting(videoURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain)
		}
		if validateErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL blocked for task %s: %v", taskID, validateErr))
			videoProxyError(c, http.StatusForbidden, "server_error", fmt.Sprintf("request blocked: %v", validateErr))
			return
		}
	}

	req.URL, err = url.Parse(videoURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse URL %s: %s", videoURL, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch video from %s: %s", videoURL, err.Error()))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	defer resp.Body.Close()
	if forwardRange {
		switch resp.StatusCode {
		case http.StatusOK, http.StatusPartialContent:
		case http.StatusRequestedRangeNotSatisfiable:
			if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
				c.Writer.Header().Set("Content-Range", contentRange)
			}
			c.Writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			c.Writer.WriteHeaderNow()
			return
		default:
			logger.LogError(c.Request.Context(), fmt.Sprintf("Upstream returned status %d for %s", resp.StatusCode, videoURL))
			videoProxyError(c, http.StatusBadGateway, "server_error",
				fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
			return
		}
		for _, key := range []string{
			"Content-Type",
			"Content-Length",
			"Content-Range",
			"Accept-Ranges",
			"Content-Disposition",
			"ETag",
			"Last-Modified",
		} {
			for _, value := range resp.Header.Values(key) {
				c.Writer.Header().Add(key, value)
			}
		}
		if resp.ContentLength >= 0 && c.Writer.Header().Get("Content-Length") == "" {
			c.Writer.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		}

		c.Writer.Header().Set("Cache-Control", "private, no-store")
		c.Writer.WriteHeader(resp.StatusCode)
		if _, err = io.Copy(c.Writer, resp.Body); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream video content: %s", err.Error()))
		}
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Upstream returned status %d for %s", resp.StatusCode, videoURL))
		videoProxyError(c, http.StatusBadGateway, "server_error",
			fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream video content: %s", err.Error()))
	}
}

func writeVideoDataURL(c *gin.Context, dataURL string) error {
	mimeType, videoBytes, err := decodeVideoDataURL(dataURL)
	if err != nil {
		return err
	}

	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write(videoBytes)
	return err
}

func writePrivateVideoDataURL(c *gin.Context, dataURL string) error {
	mimeType, videoBytes, err := decodeVideoDataURL(dataURL)
	if err != nil {
		return err
	}

	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(c.Writer, c.Request, "video", time.Time{}, bytes.NewReader(videoBytes))
	return nil
}

func decodeVideoDataURL(dataURL string) (string, []byte, error) {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid data url")
	}

	header := parts[0]
	payload := parts[1]
	if !strings.HasPrefix(header, "data:") || !strings.Contains(header, ";base64") {
		return "", nil, fmt.Errorf("unsupported data url")
	}

	mimeType := strings.TrimPrefix(header, "data:")
	mimeType = strings.TrimSuffix(mimeType, ";base64")
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	videoBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		videoBytes, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return "", nil, err
		}
	}
	return mimeType, videoBytes, nil
}

package controller

import (
	"context"
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
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func proxySeedanceVideo(c *gin.Context, task *model.Task, channel *model.Channel) {
	videoURL, err := resolveSeedanceVideoURL(task, channel, false)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Seedance video URL for task %s: %s", task.TaskID, err.Error()))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve video content")
		return
	}

	for attempt := 0; attempt < 2; attempt++ {
		resp, fetchErr := fetchSeedanceVideo(c, channel, videoURL)
		if fetchErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch Seedance video for task %s: %s", task.TaskID, fetchErr.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
			return
		}

		if attempt == 0 && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound) {
			_ = resp.Body.Close()
			videoURL, err = resolveSeedanceVideoURL(task, channel, true)
			if err != nil {
				logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to refresh Seedance video URL for task %s: %s", task.TaskID, err.Error()))
				videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to refresh video content")
				return
			}
			continue
		}

		writeSeedanceVideoResponse(c, task.TaskID, resp)
		return
	}
}

func resolveSeedanceVideoURL(task *model.Task, channel *model.Channel, forceRefresh bool) (string, error) {
	if task == nil || channel == nil {
		return "", fmt.Errorf("invalid task or channel")
	}
	if !forceRefresh && seedanceResultURLIsUsable(task.GetResultURL(), task.TaskID) {
		return strings.TrimSpace(task.GetResultURL()), nil
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}
	adaptor := relay.GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(channel.Type)))
	if adaptor == nil {
		return "", fmt.Errorf("Seedance task adaptor not found")
	}
	key := channel.Key
	if task.PrivateData.Key != "" {
		key = task.PrivateData.Key
	}
	if key == "" {
		return "", fmt.Errorf("Seedance API key is empty")
	}

	resp, err := adaptor.FetchTask(baseURL, key, task, channel.GetSetting().Proxy)
	if err != nil {
		return "", fmt.Errorf("fetch Seedance task failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Seedance task returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read Seedance task response failed: %w", err)
	}
	taskInfo, err := adaptor.ParseTaskResult(task, resp, body)
	if err != nil {
		return "", fmt.Errorf("parse Seedance task response failed: %w", err)
	}
	if taskInfo.Status != model.TaskStatusSuccess || !seedanceResultURLIsUsable(taskInfo.Url, task.TaskID) {
		return "", fmt.Errorf("Seedance task response has no usable video URL")
	}
	if err := task.UpdateResultURL(taskInfo.Url); err != nil {
		return "", fmt.Errorf("save Seedance video URL failed: %w", err)
	}
	return taskInfo.Url, nil
}

func seedanceResultURLIsUsable(resultURL string, taskID string) bool {
	parsed, err := url.Parse(strings.TrimSpace(resultURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	if strings.Contains(parsed.Path, "/v1/videos/"+taskID+"/content") {
		return false
	}
	if value := parsed.Query().Get("Expires"); value != "" {
		expiresAt, err := strconv.ParseInt(value, 10, 64)
		if err != nil || expiresAt <= time.Now().Add(time.Minute).Unix() {
			return false
		}
	}
	return true
}

func fetchSeedanceVideo(c *gin.Context, channel *model.Channel, videoURL string) (*http.Response, error) {
	proxy := channel.GetSetting().Proxy
	var client *http.Client
	var err error
	if proxy == "" {
		if err := service.ValidateSSRFProtectedFetchURL(videoURL); err != nil {
			return nil, err
		}
		client = service.GetSSRFProtectedHTTPClient()
	} else {
		fetchSetting := system_setting.GetFetchSetting()
		if err := common.ValidateURLWithFetchSetting(videoURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
			return nil, err
		}
		client, err = service.GetHttpClientWithProxy(proxy)
		if err != nil {
			return nil, err
		}
	}

	privateClient := *client
	privateClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many Seedance video redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("unsupported redirect scheme %q", req.URL.Scheme)
		}
		if proxy == "" {
			if err := service.ValidateSSRFProtectedFetchURL(req.URL.String()); err != nil {
				return err
			}
		} else {
			fetchSetting := system_setting.GetFetchSetting()
			if err := common.ValidateURLWithFetchSetting(req.URL.String(), fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
				return err
			}
		}
		if len(via) > 0 && !strings.EqualFold(via[len(via)-1].URL.Host, req.URL.Host) {
			for _, header := range []string{"Authorization", "Proxy-Authorization", "X-Api-Key", "Api-Key", "X-Goog-Api-Key", "Cookie"} {
				req.Header.Del(header)
			}
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	if value := c.GetHeader("Range"); value != "" {
		req.Header.Set("Range", value)
	}
	if value := c.GetHeader("If-Range"); value != "" {
		req.Header.Set("If-Range", value)
	}
	resp, err := privateClient.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelOnCloseReadCloser) Close() error {
	r.cancel()
	return r.ReadCloser.Close()
}

func writeSeedanceVideoResponse(c *gin.Context, taskID string, resp *http.Response) {
	defer resp.Body.Close()
	c.Writer.Header().Set("Cache-Control", "private, no-store")
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		if value := resp.Header.Get("Content-Range"); value != "" {
			c.Writer.Header().Set("Content-Range", value)
		}
		c.Writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	default:
		logger.LogError(c.Request.Context(), fmt.Sprintf("Seedance video upstream returned status %d for task %s", resp.StatusCode, taskID))
		videoProxyError(c, http.StatusBadGateway, "server_error", fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
		return
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "video/") {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Seedance video returned non-video content type %q for task %s", resp.Header.Get("Content-Type"), taskID))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Upstream service returned non-video content")
		return
	}
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Content-Disposition", "ETag", "Last-Modified"} {
		for _, value := range resp.Header.Values(key) {
			c.Writer.Header().Add(key, value)
		}
	}
	if resp.ContentLength >= 0 && c.Writer.Header().Get("Content-Length") == "" {
		c.Writer.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream Seedance video for task %s: %s", taskID, err.Error()))
	}
}

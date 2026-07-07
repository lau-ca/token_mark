package xai

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type imageRef struct {
	URL string `json:"url"`
}

type submitRequest struct {
	Model    string    `json:"model"`
	Prompt   string    `json:"prompt"`
	Image    *imageRef `json:"image,omitempty"`
	Duration int       `json:"duration,omitempty"`
}

type submitResponse struct {
	RequestID string `json:"request_id"`
}

type videoInfo struct {
	URL               string `json:"url,omitempty"`
	Duration          int    `json:"duration,omitempty"`
	RespectModeration bool   `json:"respect_moderation,omitempty"`
}

type errorInfo struct {
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

type taskResponse struct {
	ID        string     `json:"id,omitempty"`
	RequestID string     `json:"request_id,omitempty"`
	Status    string     `json:"status"`
	Video     *videoInfo `json:"video,omitempty"`
	Model     string     `json:"model,omitempty"`
	Usage     any        `json:"usage,omitempty"`
	Progress  int        `json:"progress,omitempty"`
	Error     *errorInfo `json:"error,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	duration, _ := strconv.Atoi(req.Seconds)
	if duration == 0 {
		duration = req.Duration
	}
	if duration <= 0 {
		duration = 1
	}
	return map[string]float64{"seconds": float64(duration)}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	duration, _ := strconv.Atoi(req.Seconds)
	if duration == 0 {
		duration = req.Duration
	}

	payload := submitRequest{
		Model:    info.UpstreamModelName,
		Prompt:   req.Prompt,
		Duration: duration,
	}

	imageURL, err := a.resolveImageURL(c, req)
	if err != nil {
		return nil, err
	}
	if imageURL != "" {
		payload.Image = &imageRef{URL: imageURL}
	}

	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) resolveImageURL(c *gin.Context, req relaycommon.TaskSubmitReq) (string, error) {
	if req.InputReference != "" {
		return req.InputReference, nil
	}
	if req.Image != "" {
		return req.Image, nil
	}
	if len(req.Images) > 0 {
		return req.Images[0], nil
	}

	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return "", nil
	}

	for _, field := range []string{"input_reference", "image"} {
		files := form.File[field]
		if len(files) == 0 {
			continue
		}
		fileHeader := files[0]
		file, err := fileHeader.Open()
		if err != nil {
			return "", err
		}
		fileBytes, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return "", err
		}
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = http.DetectContentType(fileBytes)
		}
		return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(fileBytes)), nil
	}
	return "", nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var xResp submitResponse
	if err := common.Unmarshal(responseBody, &xResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}
	if xResp.RequestID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("request_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	video.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, video)
	return xResp.RequestID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseUrl, "/"), taskID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var xResp taskResponse
	if err := common.Unmarshal(respBody, &xResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{Code: 0}
	switch strings.ToLower(xResp.Status) {
	case "submitted", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = taskcommon.ProgressQueued
	case "pending", "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = taskcommon.ProgressInProgress
	case "done", "completed", "succeeded", "success":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = taskcommon.ProgressComplete
		if xResp.Video != nil {
			taskResult.Url = xResp.Video.URL
		}
	case "failed", "cancelled", "canceled", "expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = taskcommon.ProgressComplete
		if xResp.Error != nil && xResp.Error.Message != "" {
			taskResult.Reason = xResp.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		return nil, fmt.Errorf("unknown task status: %s", xResp.Status)
	}

	if xResp.Progress > 0 && xResp.Progress < 100 {
		taskResult.Progress = fmt.Sprintf("%d%%", xResp.Progress)
	}
	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var xResp taskResponse
	if len(originTask.Data) > 0 {
		if err := common.Unmarshal(originTask.Data, &xResp); err != nil {
			return nil, errors.Wrap(err, "unmarshal xai task data failed")
		}
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if xResp.Video != nil {
		if xResp.Video.Duration > 0 {
			openAIVideo.Seconds = strconv.Itoa(xResp.Video.Duration)
		}
		if xResp.Video.URL != "" {
			openAIVideo.SetMetadata("url", xResp.Video.URL)
		}
	}
	if resultURL := originTask.GetResultURL(); resultURL != "" {
		openAIVideo.SetMetadata("url", resultURL)
	}
	if xResp.Error != nil && xResp.Error.Message != "" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: xResp.Error.Message,
			Code:    xResp.Error.Code,
		}
	}

	return common.Marshal(openAIVideo)
}

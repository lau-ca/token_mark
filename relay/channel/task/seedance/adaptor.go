package seedance

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type responseTask struct {
	ID          string                `json:"id"`
	TaskID      string                `json:"task_id,omitempty"`
	Object      string                `json:"object"`
	Model       string                `json:"model"`
	Status      string                `json:"status"`
	Progress    int                   `json:"progress"`
	CreatedAt   int64                 `json:"created_at"`
	CompletedAt int64                 `json:"completed_at,omitempty"`
	Seconds     string                `json:"seconds,omitempty"`
	Error       *dto.OpenAIVideoError `json:"error,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

var upstreamURLPattern = regexp.MustCompile(`(?i)(?:(?:https?|s3|gs|oss|tos)://|//)[^\s"'<>]+`)

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if info.Action != "" && info.Action != constant.TaskActionTextGenerate && info.Action != constant.TaskActionGenerate {
		return service.TaskErrorWrapperLocal(fmt.Errorf("operation is not supported by Seedance"), "unsupported_operation", http.StatusBadRequest)
	}
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/videos", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	request, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	request.Model = info.UpstreamModelName
	request.Seconds = ""
	request.Image = ""
	request.Images = nil
	request.Size = ""
	request.Mode = ""
	request.InputReference = ""
	request.Metadata = nil
	body, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	upstream := responseTask{}
	if err := common.Unmarshal(responseBody, &upstream); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrap(err, "unmarshal Seedance response"), "unmarshal_response_body_failed", http.StatusBadGateway)
	}
	upstreamTaskID := upstream.ID
	if upstreamTaskID == "" {
		upstreamTaskID = upstream.TaskID
	}
	if upstreamTaskID == "" {
		return "", nil, service.TaskErrorWrapperLocal(fmt.Errorf("upstream task_id is empty"), "invalid_response", http.StatusBadGateway)
	}

	publicResponse := dto.NewOpenAIVideo()
	publicResponse.ID = info.PublicTaskID
	publicResponse.TaskID = info.PublicTaskID
	publicResponse.Model = info.OriginModelName
	publicResponse.Status = normalizeVideoStatus(upstream.Status)
	publicResponse.Progress = normalizeProgress(upstream.Status, upstream.Progress)
	publicResponse.CreatedAt = upstream.CreatedAt
	publicResponse.CompletedAt = upstream.CompletedAt
	publicResponse.Seconds = upstream.Seconds
	if upstream.Error != nil {
		publicResponse.Error = &dto.OpenAIVideoError{
			Message: sanitizeUserText(upstream.Error.Message),
			Code:    sanitizeUserText(upstream.Error.Code),
		}
	}
	c.JSON(http.StatusOK, publicResponse)

	sanitized, err := common.Marshal(publicResponse)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	return upstreamTaskID, sanitized, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/videos/"+taskID, nil)
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
	upstream := responseTask{}
	if err := common.Unmarshal(respBody, &upstream); err != nil {
		return nil, errors.Wrap(err, "unmarshal Seedance task result")
	}

	result := &relaycommon.TaskInfo{Code: 0}
	switch upstream.Status {
	case "queued", "pending":
		result.Status = model.TaskStatusQueued
	case "processing", "in_progress":
		result.Status = model.TaskStatusInProgress
	case "completed":
		result.Status = model.TaskStatusSuccess
		result.Progress = taskcommon.ProgressComplete
	case "failed", "cancelled":
		result.Status = model.TaskStatusFailure
		result.Progress = taskcommon.ProgressComplete
		result.Reason = "task failed"
		if upstream.Error != nil && strings.TrimSpace(upstream.Error.Message) != "" {
			result.Reason = sanitizeUserText(upstream.Error.Message)
		}
	}
	if result.Status != model.TaskStatusSuccess && result.Status != model.TaskStatusFailure && upstream.Progress >= 0 && upstream.Progress < 100 {
		result.Progress = strconv.Itoa(upstream.Progress) + "%"
	}
	return result, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var response map[string]any
	if err := common.Unmarshal(task.Data, &response); err != nil {
		return nil, errors.Wrap(err, "unmarshal stored Seedance task response")
	}
	return append([]byte(nil), task.Data...), nil
}

func normalizeVideoStatus(status string) string {
	switch status {
	case "queued", "pending":
		return dto.VideoStatusQueued
	case "processing", "in_progress":
		return dto.VideoStatusInProgress
	case "completed":
		return dto.VideoStatusCompleted
	case "failed", "cancelled":
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}

func normalizeProgress(status string, progress int) int {
	if status == "completed" || status == "failed" || status == "cancelled" {
		return 100
	}
	if progress < 0 {
		return 0
	}
	if progress > 99 {
		return 99
	}
	return progress
}

func sanitizeUserText(value string) string {
	return strings.TrimSpace(upstreamURLPattern.ReplaceAllString(value, "[upstream URL hidden]"))
}

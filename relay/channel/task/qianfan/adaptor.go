package qianfan

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	channelName = "Baidu Qianfan Video"
	videoPath   = "/beta/video/generations/qianfan-video"

	metadataOperation         = "qianfan_operation"
	metadataSound             = "qianfan_sound"
	metadataHasReferenceVideo = "qianfan_has_reference_video"
	metadataHasVoice          = "qianfan_has_voice"
)

var modelList = []string{"K3.0", "k3.0", "k3.0-turbo", "K-Identify-Face", "K-Advanced-Lip-Sync"}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if strings.HasPrefix(c.Request.URL.Path, "/qianfan/") {
		return validateAdvancedRequest(c, info)
	}
	if taskErr := relaycommon.ValidateMultipartDirect(c, info); taskErr != nil {
		return taskErr
	}
	request, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	duration, err := relaycommon.ResolveTaskDuration(request)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_seconds", http.StatusBadRequest)
	}
	if duration < 0 || duration > relaycommon.MaxTaskDurationSeconds {
		return service.TaskErrorWrapperLocal(fmt.Errorf("duration must be between 1 and %d", relaycommon.MaxTaskDurationSeconds), "invalid_seconds", http.StatusBadRequest)
	}
	request.Duration = duration
	request.Seconds = ""
	if request.Metadata == nil {
		request.Metadata = make(map[string]any)
	}
	operation := "text2video"
	if len(request.Images) > 0 || strings.TrimSpace(request.Image) != "" {
		operation = "img2video"
	}
	request.Metadata[metadataOperation] = operation
	request.Metadata[metadataHasReferenceVideo] = false
	request.Metadata[metadataHasVoice] = false
	c.Set("task_request", request)
	return nil
}

func validateAdvancedRequest(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	var request qianfanRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	request.Model = strings.TrimSpace(request.Model)
	request.Type = strings.TrimSpace(request.Type)
	if request.Model == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest)
	}
	if request.ModelParameters == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model_parameters field is required"), "invalid_request", http.StatusBadRequest)
	}
	if !isAdvancedType(request.Type) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported qianfan video type %q", request.Type), "invalid_type", http.StatusBadRequest)
	}
	if err := validateModelType(request.Model, request.Type); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_type", http.StatusBadRequest)
	}

	normalized, err := normalizeAdvancedBilling(request)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	c.Set("qianfan_request", request)
	c.Set("task_request", normalized)
	info.Action = constant.TaskActionGenerate
	return nil
}

func isAdvancedType(value string) bool {
	switch value {
	case "omni-video", "motion-control", "lip-sync", "advanced-lipsync":
		return true
	default:
		return false
	}
}

func validateModelType(modelName, requestType string) error {
	if strings.EqualFold(modelName, "K-Identify-Face") && requestType != "lip-sync" {
		return fmt.Errorf("model K-Identify-Face requires type lip-sync")
	}
	if strings.EqualFold(modelName, "K-Advanced-Lip-Sync") && requestType != "advanced-lipsync" {
		return fmt.Errorf("model K-Advanced-Lip-Sync requires type advanced-lipsync")
	}
	return nil
}

func normalizeAdvancedBilling(request qianfanRequest) (relaycommon.TaskSubmitReq, error) {
	normalized := relaycommon.TaskSubmitReq{
		Model:    request.Model,
		Metadata: map[string]any{metadataOperation: request.Type},
	}
	settings, _ := request.ModelParameters["settings"].(map[string]any)

	durationValue, hasDuration := request.ModelParameters["duration"]
	if settingsDuration, ok := settings["duration"]; ok {
		durationValue, hasDuration = settingsDuration, true
	}
	if hasDuration {
		duration, err := positiveInteger(durationValue, "duration")
		if err != nil {
			return normalized, err
		}
		if duration > relaycommon.MaxTaskDurationSeconds {
			return normalized, fmt.Errorf("duration must be between 1 and %d", relaycommon.MaxTaskDurationSeconds)
		}
		normalized.Duration = duration
	}

	if resolution, exists, err := optionalStringValue(settings, request.ModelParameters, "resolution"); err != nil {
		return normalized, err
	} else if exists {
		normalized.Resolution = resolution
	}
	if mode, exists, err := optionalStringValue(settings, request.ModelParameters, "mode"); err != nil {
		return normalized, err
	} else if exists {
		normalized.Mode = mode
	}
	if sound, ok := firstValue(settings, request.ModelParameters, "sound"); ok {
		value, valid := sound.(bool)
		if !valid {
			return normalized, fmt.Errorf("sound must be a boolean")
		}
		normalized.Metadata[metadataSound] = value
	}

	normalized.Metadata[metadataHasReferenceVideo] = containsReferenceVideo(request.ModelParameters)
	normalized.Metadata[metadataHasVoice] = containsVoice(request.ModelParameters)
	return normalized, nil
}

func positiveInteger(value any, field string) (int, error) {
	switch typed := value.(type) {
	case float64:
		if typed > float64(relaycommon.MaxTaskDurationSeconds) {
			return 0, fmt.Errorf("%s must be between 1 and %d", field, relaycommon.MaxTaskDurationSeconds)
		}
		integer := int(typed)
		if typed != float64(integer) || integer <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", field)
		}
		return integer, nil
	case int:
		if typed <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", field)
		}
		return typed, nil
	default:
		return 0, fmt.Errorf("%s must be a positive integer", field)
	}
}

func firstValue(primary, fallback map[string]any, key string) (any, bool) {
	if value, ok := primary[key]; ok {
		return value, true
	}
	value, ok := fallback[key]
	return value, ok
}

func optionalStringValue(primary, fallback map[string]any, key string) (string, bool, error) {
	value, ok := firstValue(primary, fallback, key)
	if !ok {
		return "", false, nil
	}
	text, valid := value.(string)
	if !valid || strings.TrimSpace(text) == "" {
		return "", false, fmt.Errorf("%s must be a non-empty string", key)
	}
	return strings.TrimSpace(text), true, nil
}

func containsReferenceVideo(parameters map[string]any) bool {
	for _, key := range []string{"video", "video_url", "reference_video", "reference_video_url"} {
		if value, ok := parameters[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	if contents, ok := parameters["contents"].([]any); ok {
		for _, item := range contents {
			content, _ := item.(map[string]any)
			contentType, _ := content["type"].(string)
			if strings.Contains(strings.ToLower(contentType), "video") {
				return true
			}
		}
	}
	return false
}

func containsVoice(parameters map[string]any) bool {
	for _, key := range []string{"voice", "voice_id", "voice_ids"} {
		if value, ok := parameters[key]; ok && value != nil {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return true
				}
			case []any:
				if len(typed) > 0 {
					return true
				}
			}
		}
	}
	return false
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + videoPath, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	if value, ok := c.Get("qianfan_request"); ok {
		request, valid := value.(qianfanRequest)
		if !valid {
			return nil, fmt.Errorf("invalid qianfan request in context")
		}
		request.Model = info.UpstreamModelName
		if err := validateModelType(request.Model, request.Type); err != nil {
			return nil, err
		}
		data, err := common.Marshal(request)
		return bytes.NewReader(data), err
	}

	request, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	duration, err := relaycommon.ResolveTaskDuration(request)
	if err != nil {
		return nil, err
	}
	settings := map[string]any{}
	if duration > 0 {
		settings["duration"] = duration
	}
	if strings.TrimSpace(request.Resolution) != "" {
		settings["resolution"] = strings.TrimSpace(request.Resolution)
	}
	if strings.TrimSpace(request.AspectRatio) != "" {
		settings["aspect_ratio"] = strings.TrimSpace(request.AspectRatio)
	}

	parameters := map[string]any{}
	requestType := "text2video"
	imageURL := strings.TrimSpace(request.Image)
	if imageURL == "" && len(request.Images) > 0 {
		imageURL = strings.TrimSpace(request.Images[0])
	}
	if imageURL == "" {
		parameters["prompt"] = request.Prompt
	} else {
		requestType = "img2video"
		contents := make([]any, 0, 2)
		if strings.TrimSpace(request.Prompt) != "" {
			contents = append(contents, map[string]any{"type": "prompt", "text": request.Prompt})
		}
		contents = append(contents, map[string]any{"type": "first_frame", "url": imageURL})
		parameters["contents"] = contents
	}
	if len(settings) > 0 {
		parameters["settings"] = settings
	}

	data, err := common.Marshal(qianfanRequest{
		Model:           info.UpstreamModelName,
		Type:            requestType,
		ModelParameters: parameters,
	})
	return bytes.NewReader(data), err
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	var response qianfanResponse
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
	}
	if response.Code != 0 {
		return "", responseBody, service.TaskErrorWrapper(fmt.Errorf("qianfan error %d: %s", response.Code, response.Message), "task_failed", http.StatusBadRequest)
	}
	if strings.TrimSpace(response.Data.TaskID) == "" {
		return "", responseBody, service.TaskErrorWrapper(fmt.Errorf("qianfan response missing task_id"), "invalid_response", http.StatusBadGateway)
	}
	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	video.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, video)
	return response.Data.TaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	requestURL, err := url.Parse(strings.TrimRight(baseURL, "/") + videoPath)
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("task_id", taskID)
	if modelName, ok := body["model"].(string); ok && strings.TrimSpace(modelName) != "" {
		query.Set("model", modelName)
	}
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(responseBody []byte) (*relaycommon.TaskInfo, error) {
	var response qianfanResponse
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal qianfan task result: %w", err)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("qianfan error %d: %s", response.Code, response.Message)
	}
	result := &relaycommon.TaskInfo{Code: response.Code, TaskID: response.Data.TaskID, Reason: response.Data.TaskStatusMsg}
	switch response.Data.TaskStatus {
	case "submitted":
		result.Status = model.TaskStatusSubmitted
	case "processing":
		result.Status = model.TaskStatusInProgress
	case "succeed":
		result.Status = model.TaskStatusSuccess
		if len(response.Data.TaskResult.Videos) == 0 || strings.TrimSpace(response.Data.TaskResult.Videos[0].URL) == "" {
			return nil, fmt.Errorf("qianfan succeeded task missing video URL")
		}
		result.Url = response.Data.TaskResult.Videos[0].URL
	case "failed":
		result.Status = model.TaskStatusFailure
	default:
		return nil, fmt.Errorf("unknown qianfan task status %q", response.Data.TaskStatus)
	}
	return result, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var response qianfanResponse
	if len(task.Data) > 0 {
		if err := common.Unmarshal(task.Data, &response); err != nil {
			return nil, fmt.Errorf("unmarshal qianfan task data: %w", err)
		}
	}
	video := dto.NewOpenAIVideo()
	video.ID = task.TaskID
	video.TaskID = task.TaskID
	video.Status = task.Status.ToVideoStatus()
	video.SetProgressStr(task.Progress)
	video.CreatedAt = response.Data.CreatedAt
	video.CompletedAt = response.Data.UpdatedAt
	if len(response.Data.TaskResult.Videos) > 0 {
		result := response.Data.TaskResult.Videos[0]
		video.Seconds = result.Duration
		if result.URL != "" {
			video.SetMetadata("url", result.URL)
		}
	}
	if task.Status == model.TaskStatusFailure {
		video.Error = &dto.OpenAIVideoError{Message: task.FailReason}
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) GetModelList() []string {
	return modelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return channelName
}

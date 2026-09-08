package qianfan

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
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

	minimumVideoDurationSeconds = 3
	maximumVideoDurationSeconds = 15
	defaultVideoDurationSeconds = 5

	metadataOperation         = "qianfan_operation"
	metadataSound             = "qianfan_sound"
	metadataHasReferenceVideo = "qianfan_has_reference_video"
	metadataHasVoice          = "qianfan_has_voice"
)

var modelList = []string{"K3.0", "k3.0", "K3O", "k3.0-turbo", "K-Identify-Face", "K-Advanced-Lip-Sync"}

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
	if duration == 0 {
		duration = defaultVideoDurationSeconds
	}
	if duration < minimumVideoDurationSeconds || duration > maximumVideoDurationSeconds {
		return service.TaskErrorWrapperLocal(fmt.Errorf("duration must be between %d and %d", minimumVideoDurationSeconds, maximumVideoDurationSeconds), "invalid_seconds", http.StatusBadRequest)
	}
	request.Duration = duration
	request.Seconds = ""
	if err := normalizeCompatibilityQuality(&request, info.UpstreamModelName); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_quality", http.StatusBadRequest)
	}
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

func normalizeCompatibilityQuality(request *relaycommon.TaskSubmitReq, upstreamModel string) error {
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	resolution := strings.ToLower(strings.TrimSpace(request.Resolution))

	if strings.EqualFold(upstreamModel, "k3.0-turbo") {
		if resolution == "" && mode != "" {
			switch mode {
			case "std":
				resolution = "720p"
			case "pro":
				resolution = "1080p"
			default:
				return fmt.Errorf("mode %q is not supported by model k3.0-turbo", request.Mode)
			}
		}
		if resolution == "" {
			resolution = "720p"
		}
		if resolution != "720p" && resolution != "1080p" {
			return fmt.Errorf("resolution %q is not supported by model k3.0-turbo", request.Resolution)
		}
		request.Mode = ""
		request.Resolution = resolution
		return nil
	}

	if resolution != "" {
		mappedMode := ""
		switch resolution {
		case "720p":
			mappedMode = "std"
		case "1080p":
			mappedMode = "pro"
		case "4k":
			mappedMode = "4k"
		default:
			return fmt.Errorf("resolution %q is not supported by model %s", request.Resolution, upstreamModel)
		}
		if mode != "" && mode != mappedMode {
			return fmt.Errorf("mode %q conflicts with resolution %q", request.Mode, request.Resolution)
		}
		mode = mappedMode
	}
	if mode == "" {
		if strings.EqualFold(upstreamModel, "K3O") {
			mode = "pro"
		} else {
			mode = "std"
		}
	}
	if mode != "std" && mode != "pro" && mode != "4k" {
		return fmt.Errorf("mode %q is not supported by model %s", request.Mode, upstreamModel)
	}
	request.Mode = mode
	request.Resolution = ""
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
	if !isNativeType(request.Type) {
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

func isNativeType(value string) bool {
	switch value {
	case "text2video", "img2video", "omni-video", "motion-control", "lip-sync", "advanced-lipsync":
		return true
	default:
		return false
	}
}

func validateModelType(modelName, requestType string) error {
	switch {
	case strings.EqualFold(modelName, "K-Identify-Face") && requestType != "lip-sync":
		return fmt.Errorf("model K-Identify-Face requires type lip-sync")
	case strings.EqualFold(modelName, "K-Advanced-Lip-Sync") && requestType != "advanced-lipsync":
		return fmt.Errorf("model K-Advanced-Lip-Sync requires type advanced-lipsync")
	case strings.EqualFold(modelName, "K3O") && requestType != "omni-video":
		return fmt.Errorf("model K3O requires type omni-video")
	case strings.EqualFold(modelName, "k3.0-turbo") && requestType != "text2video" && requestType != "img2video":
		return fmt.Errorf("model k3.0-turbo requires type text2video or img2video")
	case (strings.EqualFold(modelName, "K3.0") || strings.EqualFold(modelName, "k3.0")) &&
		requestType != "text2video" && requestType != "img2video" && requestType != "motion-control":
		return fmt.Errorf("model K3.0 requires type text2video, img2video, or motion-control")
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
		if (request.Type == "text2video" || request.Type == "img2video" || request.Type == "omni-video") &&
			(duration < minimumVideoDurationSeconds || duration > maximumVideoDurationSeconds) {
			return normalized, fmt.Errorf("duration must be between %d and %d", minimumVideoDurationSeconds, maximumVideoDurationSeconds)
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
		value, err := soundEnabled(sound)
		if err != nil {
			return normalized, err
		}
		normalized.Metadata[metadataSound] = value
	}

	normalized.Metadata[metadataHasReferenceVideo] = containsReferenceVideo(request.ModelParameters)
	normalized.Metadata[metadataHasVoice] = containsVoice(request.ModelParameters)
	if normalized.Duration == 0 {
		switch request.Type {
		case "motion-control":
			normalized.Duration = 1
		case "advanced-lipsync":
			normalized.Duration = advancedLipSyncDuration(request.ModelParameters)
		}
	}
	return normalized, nil
}

func soundEnabled(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "on":
			return true, nil
		case "off":
			return false, nil
		}
	}
	return false, fmt.Errorf("sound must be on or off")
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
		if typed <= 0 || typed > relaycommon.MaxTaskDurationSeconds {
			return 0, fmt.Errorf("%s must be between 1 and %d", field, relaycommon.MaxTaskDurationSeconds)
		}
		return typed, nil
	case string:
		integer, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil || integer <= 0 || integer > relaycommon.MaxTaskDurationSeconds {
			return 0, fmt.Errorf("%s must be between 1 and %d", field, relaycommon.MaxTaskDurationSeconds)
		}
		return integer, nil
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
	if values, ok := parameters["video_list"].([]any); ok && len(values) > 0 {
		return true
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
	for _, key := range []string{"voice", "voice_id", "voice_ids", "voice_list"} {
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

func advancedLipSyncDuration(parameters map[string]any) int {
	faceChoose, _ := parameters["face_choose"].([]any)
	total := 0.0
	for _, item := range faceChoose {
		face, _ := item.(map[string]any)
		start, startOK := numericValue(face["sound_start_time"])
		end, endOK := numericValue(face["sound_end_time"])
		if startOK && endOK && end > start {
			total += end - start
		}
	}
	if total <= 0 {
		return 5
	}
	if total > float64(relaycommon.MaxTaskDurationSeconds) {
		return relaycommon.MaxTaskDurationSeconds
	}
	return int(math.Ceil(total))
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		value := float64(typed)
		return value, !math.IsNaN(value) && !math.IsInf(value, 0)
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		value, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
	default:
		return 0, false
	}
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
	parameters := map[string]any{}
	requestType := "text2video"
	imageURL := strings.TrimSpace(request.Image)
	if imageURL == "" && len(request.Images) > 0 {
		imageURL = strings.TrimSpace(request.Images[0])
	}
	if strings.EqualFold(info.UpstreamModelName, "k3.0-turbo") {
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
	} else {
		if strings.EqualFold(info.UpstreamModelName, "K3O") {
			requestType = "omni-video"
			parameters["sound"] = "off"
		}
		parameters["prompt"] = request.Prompt
		if imageURL != "" {
			if requestType == "omni-video" {
				parameters["image_list"] = []any{map[string]any{
					"type":      "first_frame",
					"image_url": imageURL,
				}}
			} else {
				requestType = "img2video"
				parameters["image"] = imageURL
			}
		}
		if duration > 0 {
			parameters["duration"] = strconv.Itoa(duration)
		}
		if strings.TrimSpace(request.Mode) != "" {
			parameters["mode"] = strings.TrimSpace(request.Mode)
		}
		if strings.TrimSpace(request.AspectRatio) != "" {
			parameters["aspect_ratio"] = strings.TrimSpace(request.AspectRatio)
		}
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

func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	taskID, taskData, taskErr := a.DoResponse(c, resp, info)
	if taskErr != nil {
		return nil, taskErr
	}
	return &channel.TaskSubmitResponse{UpstreamTaskID: taskID, TaskData: taskData}, nil
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
	if strings.EqualFold(info.UpstreamModelName, "K-Identify-Face") {
		if strings.TrimSpace(response.Data.SessionID) == "" {
			return "", responseBody, service.TaskErrorWrapper(fmt.Errorf("qianfan response missing session_id"), "invalid_response", http.StatusBadGateway)
		}
		c.Data(http.StatusOK, "application/json", responseBody)
		return response.Data.SessionID, responseBody, nil
	}
	if strings.TrimSpace(response.Data.TaskID) == "" {
		return "", responseBody, service.TaskErrorWrapper(fmt.Errorf("qianfan response missing task_id"), "invalid_response", http.StatusBadGateway)
	}
	upstreamTaskID := response.Data.TaskID
	if strings.HasPrefix(c.Request.URL.Path, "/qianfan/") {
		publicBody, err := replaceQianfanTaskID(responseBody, info.PublicTaskID)
		if err != nil {
			return "", responseBody, service.TaskErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
		}
		c.Data(http.StatusOK, "application/json", publicBody)
		return upstreamTaskID, responseBody, nil
	}
	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	video.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, video)
	return response.Data.TaskID, responseBody, nil
}

func replaceQianfanTaskID(responseBody []byte, taskID string) ([]byte, error) {
	var response map[string]any
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("qianfan response missing data")
	}
	data["task_id"] = taskID
	return common.Marshal(response)
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
	if response.Data.SessionID != "" && response.Data.TaskStatus == "" {
		return &relaycommon.TaskInfo{
			Code:   response.Code,
			TaskID: response.Data.SessionID,
			Status: model.TaskStatusSuccess,
		}, nil
	}
	result := &relaycommon.TaskInfo{Code: response.Code, TaskID: response.Data.TaskID, Reason: response.Data.TaskStatusMsg}
	taskStatus := strings.ToLower(strings.TrimSpace(response.Data.TaskStatus))
	if taskStatus == "" && strings.EqualFold(strings.TrimSpace(response.Message), "SUCCEED") &&
		len(response.Data.TaskResult.Videos) > 0 && strings.TrimSpace(response.Data.TaskResult.Videos[0].URL) != "" {
		taskStatus = "succeed"
	}
	switch taskStatus {
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
	if price, ok := qianfanFinalPrice(response); ok {
		result.FinalPrice = price
	}
	return result, nil
}

func qianfanFinalPrice(response qianfanResponse) (float64, bool) {
	for _, value := range []any{response.Data.FinalUnitDeduction, response.FinalUnitDeduction, response.Data.Usage.Credits, response.Usage.Credits} {
		if price, ok := numericValue(value); ok && price > 0 {
			return price, true
		}
	}
	return 0, false
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || taskResult == nil || taskResult.FinalPrice <= 0 {
		return 0
	}
	groupRatio := 1.0
	if billingContext := task.PrivateData.BillingContext; billingContext != nil {
		groupRatio = billingContext.GroupRatio
	}
	quota, clamp := common.QuotaRoundChecked(taskResult.FinalPrice * common.QuotaPerUnit * groupRatio)
	if clamp != nil && task.PrivateData.BillingContext != nil && task.PrivateData.BillingContext.QuotaClamp == nil {
		task.PrivateData.BillingContext.QuotaClamp = clamp
	}
	if quota < 0 {
		return 0
	}
	return quota
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
	video.Model = task.Properties.OriginModelName
	video.Status = task.Status.ToVideoStatus()
	video.SetProgressStr(task.Progress)
	video.CreatedAt = response.Data.CreatedAt
	if video.CreatedAt >= 1_000_000_000_000 {
		video.CreatedAt /= 1000
	}
	video.CompletedAt = response.Data.UpdatedAt
	if video.CompletedAt >= 1_000_000_000_000 {
		video.CompletedAt /= 1000
	}
	if len(response.Data.TaskResult.Videos) > 0 {
		result := response.Data.TaskResult.Videos[0]
		video.Seconds = result.Duration
		if result.URL != "" {
			video.URL = result.URL
			video.VideoURL = result.URL
			video.SetMetadata("url", result.URL)
		}
	}
	if task.Status == model.TaskStatusFailure {
		video.Error = &dto.OpenAIVideoError{Message: task.FailReason}
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) ConvertToNativeVideo(task *model.Task) ([]byte, error) {
	if len(task.Data) == 0 {
		return nil, fmt.Errorf("qianfan task data is empty")
	}
	return replaceQianfanTaskID(task.Data, task.TaskID)
}

func (a *TaskAdaptor) GetModelList() []string {
	return modelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return channelName
}

package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type      string     `json:"type,omitempty"`
	Text      string     `json:"text,omitempty"`
	ImageURL  *MediaURL  `json:"image_url,omitempty"`
	VideoURL  *MediaURL  `json:"video_url,omitempty"`
	AudioURL  *MediaURL  `json:"audio_url,omitempty"`
	DraftTask *DraftTask `json:"draft_task,omitempty"`
	Role      string     `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type DraftTask struct {
	ID string `json:"id,omitempty"`
}

type requestPayload struct {
	Model                 string        `json:"model"`
	Content               []ContentItem `json:"content,omitempty"`
	CallbackURL           string        `json:"callback_url,omitempty"`
	ReturnLastFrame       *bool         `json:"return_last_frame,omitempty"`
	ServiceTier           string        `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *int          `json:"execution_expires_after,omitempty"`
	GenerateAudio         *bool         `json:"generate_audio,omitempty"`
	Draft                 *bool         `json:"draft,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	SafetyIdentifier string `json:"safety_identifier,omitempty"`
	Priority         *int   `json:"priority,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	Ratio            string `json:"ratio,omitempty"`
	Duration         *int   `json:"duration,omitempty"`
	Frames           *int   `json:"frames,omitempty"`
	Seed             *int   `json:"seed,omitempty"`
	CameraFixed      *bool  `json:"camera_fixed,omitempty"`
	Watermark        *bool  `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID                    string           `json:"id"`
	Model                 string           `json:"model"`
	Status                string           `json:"status"`
	Content               *responseContent `json:"content"`
	Seed                  int              `json:"seed"`
	Resolution            string           `json:"resolution"`
	Duration              int              `json:"duration"`
	Ratio                 string           `json:"ratio"`
	FramesPerSecond       int              `json:"framespersecond"`
	Priority              int              `json:"priority"`
	Draft                 bool             `json:"draft"`
	GenerateAudio         bool             `json:"generate_audio"`
	ServiceTier           string           `json:"service_tier"`
	ExecutionExpiresAfter int              `json:"execution_expires_after"`
	Tools                 []struct {
		Type string `json:"type"`
	} `json:"tools"`
	Usage *struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		ToolUsage        struct {
			WebSearch int `json:"web_search"`
		} `json:"tool_usage"`
	} `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type responseContent struct {
	VideoURL     string `json:"video_url"`
	LastFrameURL string `json:"last_frame_url"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *taskdto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return strings.TrimRight(a.baseURL, "/") + taskPath(a.ChannelType), nil
}

func taskPath(channelType int) string {
	if channelType == constant.ChannelTypeDoubaoVideo {
		return "/v3/contents/generations/tasks"
	}
	return "/api/v3/contents/generations/tasks"
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateBilling 根据请求 metadata 中的输出分辨率与是否包含视频输入，返回相对基准价的计费 OtherRatio。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	hasVideo := req.HasReferenceVideo()
	resolution := req.Resolution
	if resolution == "" {
		resolution, _ = req.Metadata["resolution"].(string)
	}
	ratio, ok := GetVideoInputRatio(info.OriginModelName, resolution, hasVideo)
	if !ok || ratio == 1.0 {
		return nil
	}
	return map[string]float64{"video_input": ratio}
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
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

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()
	taskData = responseBody

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	if c.Request.URL.Path == "/v3/contents/generations/tasks" {
		c.JSON(http.StatusOK, responsePayload{ID: dResp.ID})
		initialTask := responseTask{
			ID:        dResp.ID,
			Model:     info.OriginModelName,
			Status:    "queued",
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		}
		taskData, err = common.Marshal(initialTask)
		if err != nil {
			taskErr = service.TaskErrorWrapper(err, "marshal_task_data_failed", http.StatusInternalServerError)
			return
		}
	} else {
		c.JSON(http.StatusOK, ov)
	}
	return dResp.ID, taskData, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s%s/%s", strings.TrimRight(baseUrl, "/"), taskPath(a.ChannelType), taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) DeleteTask(baseURL, key, taskID, proxy string) (*http.Response, error) {
	uri := fmt.Sprintf("%s%s/%s", strings.TrimRight(baseURL, "/"), taskPath(a.ChannelType), taskID)
	req, err := http.NewRequest(http.MethodDelete, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
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

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{Model: req.Model}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	r.Model = req.Model

	if len(req.Content) > 0 {
		r.Content = make([]ContentItem, 0, len(req.Content))
		for _, item := range req.Content {
			content := ContentItem{Type: item.Type, Text: item.Text, Role: item.Role}
			if item.ImageURL != nil {
				content.ImageURL = &MediaURL{URL: item.ImageURL.URL}
			}
			if item.VideoURL != nil {
				content.VideoURL = &MediaURL{URL: item.VideoURL.URL}
			}
			if item.AudioURL != nil {
				content.AudioURL = &MediaURL{URL: item.AudioURL.URL}
			}
			if item.DraftTask != nil {
				content.DraftTask = &DraftTask{ID: item.DraftTask.ID}
			}
			r.Content = append(r.Content, content)
		}
	} else {
		r.Content = nil

		for _, imgURL := range req.Images {
			r.Content = append(r.Content, ContentItem{
				Type:     "image_url",
				ImageURL: &MediaURL{URL: imgURL},
			})
		}
		for _, imgURL := range req.ReferenceImages {
			r.Content = append(r.Content, ContentItem{Type: "image_url", ImageURL: &MediaURL{URL: imgURL}, Role: "reference_image"})
		}
		for _, videoURL := range req.ReferenceVideos {
			r.Content = append(r.Content, ContentItem{Type: "video_url", VideoURL: &MediaURL{URL: videoURL}, Role: "reference_video"})
		}
		for _, audioURL := range req.ReferenceAudios {
			r.Content = append(r.Content, ContentItem{Type: "audio_url", AudioURL: &MediaURL{URL: audioURL}, Role: "reference_audio"})
		}
		r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
		if strings.TrimSpace(req.Prompt) != "" {
			r.Content = append(r.Content, ContentItem{Type: "text", Text: req.Prompt})
		}
	}

	r.CallbackURL = taskcommon.DefaultString(req.CallbackURL, r.CallbackURL)
	r.ServiceTier = taskcommon.DefaultString(req.ServiceTier, r.ServiceTier)
	r.SafetyIdentifier = taskcommon.DefaultString(req.SafetyIdentifier, r.SafetyIdentifier)
	r.Resolution = taskcommon.DefaultString(req.Resolution, r.Resolution)
	r.Ratio = taskcommon.DefaultString(req.Ratio, r.Ratio)
	if req.ReturnLastFrame != nil {
		r.ReturnLastFrame = req.ReturnLastFrame
	}
	if req.ExecutionExpiresAfter != nil {
		r.ExecutionExpiresAfter = req.ExecutionExpiresAfter
	}
	if req.GenerateAudio != nil {
		r.GenerateAudio = req.GenerateAudio
	}
	if req.Draft != nil {
		r.Draft = req.Draft
	}
	if len(req.Tools) > 0 {
		r.Tools = make([]struct {
			Type string `json:"type,omitempty"`
		}, len(req.Tools))
		for i, tool := range req.Tools {
			r.Tools[i].Type = tool.Type
		}
	}
	if req.Priority != nil {
		r.Priority = req.Priority
	}
	if req.Frames != nil {
		r.Frames = req.Frames
	}
	if req.Seed != nil {
		r.Seed = req.Seed
	}
	if req.CameraFixed != nil {
		r.CameraFixed = req.CameraFixed
	}
	if req.Watermark != nil {
		r.Watermark = req.Watermark
	}
	if req.Duration != 0 || req.Seconds == "" {
		if req.Duration != 0 {
			r.Duration = lo.ToPtr(req.Duration)
		}
	}
	if sec, err := strconv.Atoi(req.Seconds); err == nil && req.Seconds != "" {
		r.Duration = lo.ToPtr(sec)
	}

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// Map Doubao status to internal status
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if resTask.Content != nil {
			taskResult.Url = resTask.Content.VideoURL
		}
		// 解析 usage 信息用于按倍率计费
		if resTask.Usage != nil {
			taskResult.CompletionTokens = resTask.Usage.CompletionTokens
			taskResult.TotalTokens = resTask.Usage.TotalTokens
		}
	case "failed", "cancelled", "expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if resTask.Error != nil {
			taskResult.Reason = resTask.Error.Message
		}
		if taskResult.Reason == "" {
			taskResult.Reason = resTask.Status
		}
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	if dResp.Content != nil {
		openAIVideo.SetMetadata("url", dResp.Content.VideoURL)
		if dResp.Content.LastFrameURL != "" {
			openAIVideo.SetMetadata("last_frame_url", dResp.Content.LastFrameURL)
		}
	}
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if dResp.Status == "failed" || dResp.Status == "cancelled" || dResp.Status == "expired" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: dResp.Status,
		}
		if dResp.Error != nil {
			openAIVideo.Error.Message = dResp.Error.Message
			openAIVideo.Error.Code = dResp.Error.Code
		}
	}

	return common.Marshal(openAIVideo)
}

func (a *TaskAdaptor) ConvertToNativeVideo(originTask *model.Task) ([]byte, error) {
	var response responseTask
	if err := common.Unmarshal(originTask.Data, &response); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}
	response.ID = originTask.TaskID
	if response.Model == "" {
		response.Model = originTask.Properties.OriginModelName
	}
	response.Status = normalizeNativeStatus(response.Status, originTask.Status)
	return common.Marshal(response)
}

func normalizeNativeStatus(status string, localStatus model.TaskStatus) string {
	switch status {
	case "pending":
		return "queued"
	case "processing":
		return "running"
	case "queued", "running", "succeeded", "failed", "cancelled", "expired":
		return status
	}
	switch localStatus {
	case model.TaskStatusQueued, model.TaskStatusSubmitted:
		return "queued"
	case model.TaskStatusInProgress:
		return "running"
	case model.TaskStatusSuccess:
		return "succeeded"
	case model.TaskStatusFailure:
		return "failed"
	default:
		return status
	}
}

package common

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func SanitizeURLForLog(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	if len(query) == 0 {
		return rawURL
	}

	changed := false
	for key := range query {
		if isSensitiveURLQueryKey(key) {
			query.Set(key, "***masked***")
			changed = true
		}
	}
	if !changed {
		return rawURL
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func isSensitiveURLQueryKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "key",
		"api_key",
		"api-key",
		"apikey",
		"x-api-key",
		"access_token",
		"refresh_token",
		"id_token",
		"token",
		"authorization",
		"auth",
		"client_secret",
		"secret",
		"password",
		"passwd",
		"signature",
		"sig",
		"awsaccesskeyid",
		"x-amz-credential",
		"x-amz-security-token",
		"x-amz-signature":
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "signature")
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	info.Action = action
	c.Set("task_request", requestObj)
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	req, ok := v.(TaskSubmitReq)
	if !ok {
		return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
	}
	return req, nil
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

// MaxTaskDurationSeconds caps user-supplied video duration. Duration is used
// as a billing multiplier (OtherRatio "seconds"); an unbounded value could
// overflow quota calculation into a negative charge.
const MaxTaskDurationSeconds = 3600

const (
	SeedanceVideoModelFast     = "videos-fast"
	SeedanceVideoModelMini     = "videos-mini"
	SeedanceVideoModelStandard = "videos-standard"

	minSeedanceVideoDurationSeconds  = 4
	maxSeedanceVideoDurationSeconds  = 15
	minSeedanceExecutionExpiresAfter = 3600
	maxSeedanceExecutionExpiresAfter = 259200
)

var nativeSeedanceResolutions = map[string]struct{}{
	"480p":  {},
	"720p":  {},
	"1080p": {},
	"4k":    {},
}

var nativeSeedanceRatios = map[string]struct{}{
	"16:9":     {},
	"4:3":      {},
	"1:1":      {},
	"3:4":      {},
	"9:16":     {},
	"21:9":     {},
	"adaptive": {},
}

func IsSeedanceVideoModel(model string) bool {
	switch model {
	case SeedanceVideoModelFast, SeedanceVideoModelMini, SeedanceVideoModelStandard:
		return true
	default:
		return false
	}
}

func ResolveSeedanceVideoDuration(req TaskSubmitReq) (int, error) {
	duration, err := ResolveTaskDuration(req)
	if err != nil {
		return 0, err
	}
	if !req.durationProvided && req.Duration == 0 && req.Seconds == "" {
		duration = minSeedanceVideoDurationSeconds
	}

	if duration < minSeedanceVideoDurationSeconds || duration > maxSeedanceVideoDurationSeconds {
		return 0, fmt.Errorf("seconds must be between %d and %d", minSeedanceVideoDurationSeconds, maxSeedanceVideoDurationSeconds)
	}
	return duration, nil
}

func ResolveTaskDuration(req TaskSubmitReq) (int, error) {
	if req.durationParseErr != nil {
		return 0, req.durationParseErr
	}

	duration := req.Duration
	hasDuration := req.durationProvided || req.Duration != 0
	if req.Seconds == "" {
		return duration, nil
	}

	seconds, err := strconv.Atoi(req.Seconds)
	if err != nil {
		return 0, fmt.Errorf("seconds must be an integer")
	}
	if hasDuration && seconds != duration {
		return 0, fmt.Errorf("duration and seconds must match when both are provided")
	}
	return seconds, nil
}

func validateTaskDurationBounds(req TaskSubmitReq) *dto.TaskError {
	seconds := req.Duration
	if seconds == 0 && req.Seconds != "" {
		seconds, _ = strconv.Atoi(req.Seconds)
	}
	if seconds < 0 || seconds > MaxTaskDurationSeconds {
		return createTaskError(fmt.Errorf("seconds must be between 1 and %d", MaxTaskDurationSeconds), "invalid_seconds", http.StatusBadRequest, true)
	}
	return nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, error) {
	var req TaskSubmitReq
	if _, err := c.MultipartForm(); err != nil {
		return req, err
	}

	formData := c.Request.PostForm
	req = TaskSubmitReq{
		Prompt:   formData.Get("prompt"),
		Model:    formData.Get("model"),
		Mode:     formData.Get("mode"),
		Image:    formData.Get("image"),
		Size:     formData.Get("size"),
		Metadata: make(map[string]interface{}),
	}

	if durationStr := formData.Get("seconds"); durationStr != "" {
		if duration, err := strconv.Atoi(durationStr); err == nil {
			req.Duration = duration
		}
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}
	return ValidateParsedTaskRequest(c, info, req)
}

func ValidateParsedTaskRequest(c *gin.Context, info *RelayInfo, req TaskSubmitReq) *dto.TaskError {
	prompt := req.Prompt
	model := req.Model
	size := req.Size
	seconds, _ := strconv.Atoi(req.Seconds)
	var hasInputReference bool
	if strings.TrimSpace(prompt) == "" {
		var texts []string
		for _, item := range req.Content {
			if strings.TrimSpace(item.Text) != "" {
				texts = append(texts, item.Text)
			}
		}
		prompt = strings.Join(texts, "\n")
		req.Prompt = prompt
	}
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = []string{req.InputReference}
	} else if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{strings.TrimSpace(req.Image)}
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasImage() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	if IsSeedanceVideoModel(model) {
		if len(req.ReferenceImages) > 0 {
			hasInputReference = true
		}

		duration, err := ResolveSeedanceVideoDuration(req)
		if err != nil {
			return createTaskError(err, "invalid_seconds", http.StatusBadRequest, true)
		}

		resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
		isSupportedResolution := false
		switch model {
		case SeedanceVideoModelFast, SeedanceVideoModelMini:
			isSupportedResolution = lo.Contains([]string{"480p", "720p"}, resolution)
		case SeedanceVideoModelStandard:
			isSupportedResolution = lo.Contains([]string{"480p", "720p", "1080p", "4k"}, resolution)
		}
		if !isSupportedResolution {
			return createTaskError(fmt.Errorf("resolution %q is not supported by model %s", req.Resolution, model), "invalid_resolution", http.StatusBadRequest, true)
		}

		req.Duration = duration
		req.Seconds = ""
		req.Resolution = resolution
	} else if taskErr := validateTaskDurationBounds(req); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextGenerate
	if IsSeedanceVideoModel(model) || hasInputReference {
		action = constant.TaskActionGenerate
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":          true,
		"model":           true,
		"mode":            true,
		"image":           true,
		"images":          true,
		"size":            true,
		"duration":        true,
		"input_reference": true, // Sora 特有字段
	}
	return knownFields[field]
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	var err error
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, err = validateMultipartTaskRequest(c, info, action)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	}
	// 为了metadata字段的兼容性，统一UnmarshalBodyReusable
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}
	if c.Request.Method == http.MethodPost && c.Request.URL.Path == "/v3/contents/generations/tasks" {
		return validateNativeSeedanceTaskRequest(c, info, req, action)
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if taskErr := validateTaskDurationBounds(req); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}

	storeTaskRequest(c, info, action, req)
	return nil
}

func validateNativeSeedanceTaskRequest(c *gin.Context, info *RelayInfo, req TaskSubmitReq, action string) *dto.TaskError {
	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	hasMeaningfulContent := false
	for _, item := range req.Content {
		switch item.Type {
		case "text":
			hasMeaningfulContent = hasMeaningfulContent || strings.TrimSpace(item.Text) != ""
		case "image_url":
			hasMeaningfulContent = hasMeaningfulContent || item.ImageURL != nil && strings.TrimSpace(item.ImageURL.URL) != ""
		case "video_url":
			hasMeaningfulContent = hasMeaningfulContent || item.VideoURL != nil && strings.TrimSpace(item.VideoURL.URL) != ""
		case "audio_url":
			if item.AudioURL == nil || strings.TrimSpace(item.AudioURL.URL) == "" {
				return createTaskError(fmt.Errorf("audio_url content requires url"), "invalid_content", http.StatusBadRequest, true)
			}
		case "draft_task":
			if item.DraftTask == nil || strings.TrimSpace(item.DraftTask.ID) == "" {
				return createTaskError(fmt.Errorf("draft_task content requires id"), "invalid_content", http.StatusBadRequest, true)
			}
		default:
			return createTaskError(fmt.Errorf("unsupported content type %q", item.Type), "invalid_content", http.StatusBadRequest, true)
		}
	}
	if !hasMeaningfulContent {
		return createTaskError(fmt.Errorf("content must include text, image_url, or video_url"), "invalid_content", http.StatusBadRequest, true)
	}

	if req.durationParseErr != nil {
		return createTaskError(req.durationParseErr, "invalid_duration", http.StatusBadRequest, true)
	}
	if req.durationProvided && req.Duration != -1 && (req.Duration < minSeedanceVideoDurationSeconds || req.Duration > maxSeedanceVideoDurationSeconds) {
		return createTaskError(fmt.Errorf("duration must be -1 or between %d and %d", minSeedanceVideoDurationSeconds, maxSeedanceVideoDurationSeconds), "invalid_duration", http.StatusBadRequest, true)
	}
	if req.ExecutionExpiresAfter != nil && (*req.ExecutionExpiresAfter < minSeedanceExecutionExpiresAfter || *req.ExecutionExpiresAfter > maxSeedanceExecutionExpiresAfter) {
		return createTaskError(fmt.Errorf("execution_expires_after must be between %d and %d", minSeedanceExecutionExpiresAfter, maxSeedanceExecutionExpiresAfter), "invalid_execution_expires_after", http.StatusBadRequest, true)
	}
	if req.Priority != nil && (*req.Priority < 0 || *req.Priority > 9) {
		return createTaskError(fmt.Errorf("priority must be between 0 and 9"), "invalid_priority", http.StatusBadRequest, true)
	}
	if req.Resolution != "" {
		req.Resolution = strings.ToLower(strings.TrimSpace(req.Resolution))
		if _, ok := nativeSeedanceResolutions[req.Resolution]; !ok {
			return createTaskError(fmt.Errorf("unsupported resolution %q", req.Resolution), "invalid_resolution", http.StatusBadRequest, true)
		}
	}
	if req.Ratio != "" {
		req.Ratio = strings.ToLower(strings.TrimSpace(req.Ratio))
		if _, ok := nativeSeedanceRatios[req.Ratio]; !ok {
			return createTaskError(fmt.Errorf("unsupported ratio %q", req.Ratio), "invalid_ratio", http.StatusBadRequest, true)
		}
	}

	storeTaskRequest(c, info, action, req)
	return nil
}

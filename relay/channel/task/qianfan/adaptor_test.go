package qianfan

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newQianfanCompatibilityContext(t *testing.T, body, upstreamModel string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	return context, &relaycommon.RelayInfo{
		OriginModelName: upstreamModel,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task-public"},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: upstreamModel,
		},
	}
}

func TestValidateModelType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model       string
		requestType string
		wantError   bool
	}{
		{model: "K3.0", requestType: "text2video"},
		{model: "K3.0", requestType: "img2video"},
		{model: "K3.0", requestType: "motion-control"},
		{model: "K3O", requestType: "omni-video"},
		{model: "k3.0-turbo", requestType: "text2video"},
		{model: "k3.0-turbo", requestType: "img2video"},
		{model: "K-Identify-Face", requestType: "lip-sync"},
		{model: "K-Advanced-Lip-Sync", requestType: "advanced-lipsync"},
		{model: "K3O", requestType: "text2video", wantError: true},
		{model: "K3.0", requestType: "omni-video", wantError: true},
		{model: "k3.0-turbo", requestType: "motion-control", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.model+"/"+test.requestType, func(t *testing.T) {
			err := validateModelType(test.model, test.requestType)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNormalizeAdvancedBillingUsesOfficialFields(t *testing.T) {
	t.Parallel()

	request := qianfanRequest{
		Model: "K3O",
		Type:  "omni-video",
		ModelParameters: map[string]any{
			"duration":   float64(8),
			"mode":       "pro",
			"sound":      "on",
			"video_list": []any{map[string]any{"video_url": "https://example.com/reference.mp4"}},
			"voice_list": []any{map[string]any{"voice_id": "voice-1"}},
		},
	}

	normalized, err := normalizeAdvancedBilling(request)
	require.NoError(t, err)
	assert.Equal(t, 8, normalized.Duration)
	assert.Equal(t, "pro", normalized.Mode)
	assert.Equal(t, true, normalized.Metadata[metadataSound])
	assert.Equal(t, true, normalized.Metadata[metadataHasReferenceVideo])
	assert.Equal(t, true, normalized.Metadata[metadataHasVoice])
}

func TestNormalizeAdvancedBillingAcceptsOfficialStringDuration(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeAdvancedBilling(qianfanRequest{
		Model: "K3O",
		Type:  "omni-video",
		ModelParameters: map[string]any{
			"duration": "3",
			"mode":     "std",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 3, normalized.Duration)
}

func TestNormalizeAdvancedBillingRejectsUnsupportedGenerationDuration(t *testing.T) {
	t.Parallel()

	for _, duration := range []string{"2", "16"} {
		_, err := normalizeAdvancedBilling(qianfanRequest{
			Model: "K3O",
			Type:  "omni-video",
			ModelParameters: map[string]any{
				"duration": duration,
				"mode":     "std",
			},
		})
		require.Error(t, err)
	}
}

func TestValidateCompatibilityRequestNormalizesOfficialQualityFields(t *testing.T) {
	tests := []struct {
		name           string
		upstreamModel  string
		body           string
		wantMode       string
		wantResolution string
	}{
		{
			name:          "K3 resolution compatibility",
			upstreamModel: "K3.0",
			body:          `{"model":"Kling 3.0","prompt":"lake","duration":3,"resolution":"1080p"}`,
			wantMode:      "pro",
		},
		{
			name:          "Omni resolution compatibility",
			upstreamModel: "K3O",
			body:          `{"model":"Kling 3.0 Omni","prompt":"lake","duration":3,"resolution":"720p"}`,
			wantMode:      "std",
		},
		{
			name:          "Omni official default",
			upstreamModel: "K3O",
			body:          `{"model":"Kling 3.0 Omni","prompt":"lake","duration":3}`,
			wantMode:      "pro",
		},
		{
			name:           "Turbo resolution",
			upstreamModel:  "k3.0-turbo",
			body:           `{"model":"Kling 3.0 Turbo","prompt":"lake","duration":3,"resolution":"1080p"}`,
			wantResolution: "1080p",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, info := newQianfanCompatibilityContext(t, test.body, test.upstreamModel)
			adaptor := &TaskAdaptor{}
			require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
			request, err := relaycommon.GetTaskRequest(context)
			require.NoError(t, err)
			assert.Equal(t, test.wantMode, request.Mode)
			assert.Equal(t, test.wantResolution, request.Resolution)
		})
	}
}

func TestValidateCompatibilityRequestRejectsUnsupportedValues(t *testing.T) {
	tests := []struct {
		name          string
		upstreamModel string
		body          string
	}{
		{name: "duration below minimum", upstreamModel: "K3.0", body: `{"model":"Kling 3.0","prompt":"lake","duration":2,"mode":"std"}`},
		{name: "duration above maximum", upstreamModel: "K3.0", body: `{"model":"Kling 3.0","prompt":"lake","duration":16,"mode":"std"}`},
		{name: "invalid K3 mode", upstreamModel: "K3.0", body: `{"model":"Kling 3.0","prompt":"lake","duration":3,"mode":"cinema"}`},
		{name: "invalid K3 resolution", upstreamModel: "K3.0", body: `{"model":"Kling 3.0","prompt":"lake","duration":3,"resolution":"999p"}`},
		{name: "invalid Turbo resolution", upstreamModel: "k3.0-turbo", body: `{"model":"Kling 3.0 Turbo","prompt":"lake","duration":3,"resolution":"999p"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, info := newQianfanCompatibilityContext(t, test.body, test.upstreamModel)
			taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		})
	}
}

func TestNormalizeAdvancedLipSyncBillingDuration(t *testing.T) {
	t.Parallel()

	request := qianfanRequest{
		Model: "K-Advanced-Lip-Sync",
		Type:  "advanced-lipsync",
		ModelParameters: map[string]any{
			"face_choose": []any{
				map[string]any{"sound_start_time": float64(1), "sound_end_time": float64(4.2)},
				map[string]any{"sound_start_time": float64(0), "sound_end_time": float64(2.1)},
			},
		},
	}

	normalized, err := normalizeAdvancedBilling(request)
	require.NoError(t, err)
	assert.Equal(t, 6, normalized.Duration)
}

func TestNormalizeAdvancedBillingRejectsOversizedIntegerDuration(t *testing.T) {
	t.Parallel()

	_, err := normalizeAdvancedBilling(qianfanRequest{
		Model: "K3.0",
		Type:  "text2video",
		ModelParameters: map[string]any{
			"duration": relaycommon.MaxTaskDurationSeconds + 1,
		},
	})
	require.Error(t, err)
}

func TestBuildRequestBodyUsesStandardK3Shape(t *testing.T) {
	t.Parallel()

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:      "make a video",
		Image:       "https://example.com/first.png",
		Duration:    5,
		Mode:        "pro",
		AspectRatio: "16:9",
	})
	body, err := (&TaskAdaptor{}).BuildRequestBody(context, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "K3.0"},
	})
	require.NoError(t, err)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var request qianfanRequest
	require.NoError(t, common.Unmarshal(data, &request))
	assert.Equal(t, "img2video", request.Type)
	assert.Equal(t, "https://example.com/first.png", request.ModelParameters["image"])
	assert.Equal(t, "5", request.ModelParameters["duration"])
	assert.Equal(t, "pro", request.ModelParameters["mode"])
	assert.NotContains(t, request.ModelParameters, "settings")
	assert.NotContains(t, request.ModelParameters, "contents")
}

func TestBuildRequestBodyUsesOmniShape(t *testing.T) {
	t.Parallel()

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:      "make an omni video",
		Duration:    3,
		Mode:        "std",
		AspectRatio: "16:9",
	})
	body, err := (&TaskAdaptor{}).BuildRequestBody(context, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "K3O"},
	})
	require.NoError(t, err)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var request qianfanRequest
	require.NoError(t, common.Unmarshal(data, &request))
	assert.Equal(t, "omni-video", request.Type)
	assert.Equal(t, "make an omni video", request.ModelParameters["prompt"])
	assert.Equal(t, "3", request.ModelParameters["duration"])
	assert.Equal(t, "std", request.ModelParameters["mode"])
	assert.Equal(t, "16:9", request.ModelParameters["aspect_ratio"])
	assert.Equal(t, "off", request.ModelParameters["sound"])
}

func TestBuildRequestBodyUsesOfficialOmniImageList(t *testing.T) {
	t.Parallel()

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:   "animate the first frame",
		Image:    "https://example.com/first.png",
		Duration: 3,
		Mode:     "std",
	})
	body, err := (&TaskAdaptor{}).BuildRequestBody(context, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "K3O"},
	})
	require.NoError(t, err)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var request qianfanRequest
	require.NoError(t, common.Unmarshal(data, &request))
	imageList, ok := request.ModelParameters["image_list"].([]any)
	require.True(t, ok)
	require.Len(t, imageList, 1)
	image, ok := imageList[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "first_frame", image["type"])
	assert.Equal(t, "https://example.com/first.png", image["image_url"])
	assert.NotContains(t, request.ModelParameters, "image")
}

func TestBuildRequestBodyUsesTurboShape(t *testing.T) {
	t.Parallel()

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:      "make a video",
		Image:       "https://example.com/first.png",
		Duration:    5,
		Resolution:  "720p",
		AspectRatio: "16:9",
	})
	body, err := (&TaskAdaptor{}).BuildRequestBody(context, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "k3.0-turbo"},
	})
	require.NoError(t, err)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var request qianfanRequest
	require.NoError(t, common.Unmarshal(data, &request))
	assert.Equal(t, "img2video", request.Type)
	assert.Contains(t, request.ModelParameters, "contents")
	settings, ok := request.ModelParameters["settings"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(5), settings["duration"])
	assert.Equal(t, "720p", settings["resolution"])
	assert.NotContains(t, request.ModelParameters, "image")
}

func TestDoResponseSupportsSynchronousFaceIdentification(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/qianfan/v1/videos", nil)
	responseBody := []byte(`{"code":0,"message":"success","data":{"session_id":"session-1","face_data":[{"face_id":"face-1"}],"final_unit_deduction":"0.05"}}`)
	response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(responseBody))}

	taskID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(context, response, &relaycommon.RelayInfo{
		OriginModelName: "K-Identify-Face",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "K-Identify-Face"},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task-public"},
	})

	require.Nil(t, taskErr)
	assert.Equal(t, "session-1", taskID)
	assert.JSONEq(t, string(responseBody), string(taskData))
	assert.JSONEq(t, string(responseBody), recorder.Body.String())

	result, err := (&TaskAdaptor{}).ParseTaskResult(taskData)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
}

func TestParseTaskResultReadsFinalPrice(t *testing.T) {
	t.Parallel()

	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"code":0,
		"data":{
			"task_id":"upstream-task",
			"task_status":"succeed",
			"usage":{"credits":7.2},
			"task_result":{"videos":[{"url":"https://example.com/video.mp4"}]}
		}
	}`))
	require.NoError(t, err)
	assert.Equal(t, 7.2, result.FinalPrice)
	assert.Equal(t, "https://example.com/video.mp4", result.Url)
}

func TestParseTaskResultAcceptsTurboSuccessMessage(t *testing.T) {
	t.Parallel()

	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"code":0,
		"message":"SUCCEED",
		"data":{
			"task_id":"upstream-task",
			"task_status":"",
			"task_result":{"videos":[{"duration":"3.041","url":"https://example.com/turbo.mp4"}]}
		},
		"usage":{"credits":2.4}
	}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
	assert.Equal(t, "https://example.com/turbo.mp4", result.Url)
	assert.Equal(t, 2.4, result.FinalPrice)
}

func TestConvertToOpenAIVideoReturnsCompletePublicResponse(t *testing.T) {
	t.Parallel()

	task := &model.Task{
		TaskID:   "task-public",
		Status:   model.TaskStatusSuccess,
		Progress: "100%",
		Properties: model.Properties{
			OriginModelName: "Kling 3.0",
		},
		Data: []byte(`{
			"code":0,
			"data":{
				"created_at":1786287263000,
				"updated_at":1786287328000,
				"task_result":{"videos":[{"duration":"3.041","url":"https://example.com/video.mp4"}]}
			}
		}`),
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	var response dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &response))
	assert.Equal(t, "Kling 3.0", response.Model)
	assert.Equal(t, "https://example.com/video.mp4", response.URL)
	assert.Equal(t, "https://example.com/video.mp4", response.VideoURL)
	assert.Equal(t, "https://example.com/video.mp4", response.Metadata["url"])
	assert.Equal(t, int64(1786287263), response.CreatedAt)
	assert.Equal(t, int64(1786287328), response.CompletedAt)
}

func TestReplaceQianfanTaskIDPreservesProviderFields(t *testing.T) {
	t.Parallel()

	body, err := replaceQianfanTaskID([]byte(`{
		"code":0,
		"request_id":"request-1",
		"data":{"task_id":"upstream-task","provider_extension":{"value":true}}
	}`), "task-public")
	require.NoError(t, err)

	var response map[string]any
	require.NoError(t, common.Unmarshal(body, &response))
	assert.Equal(t, "request-1", response["request_id"])
	data, ok := response["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "task-public", data["task_id"])
	assert.Contains(t, data, "provider_extension")
}

func TestAdjustBillingOnCompleteUsesFinalPriceAndGroupRatio(t *testing.T) {
	t.Parallel()

	task := &model.Task{}
	task.PrivateData.BillingContext = &model.TaskBillingContext{GroupRatio: 1.5}
	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{FinalPrice: 2})
	assert.Equal(t, common.QuotaRound(2*common.QuotaPerUnit*1.5), quota)
}

func TestModelListContainsK3O(t *testing.T) {
	t.Parallel()
	assert.Contains(t, (&TaskAdaptor{}).GetModelList(), "K3O")
}

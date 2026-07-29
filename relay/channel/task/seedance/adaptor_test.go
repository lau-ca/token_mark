package seedance

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSeedanceContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		OriginModelName: relaycommon.SeedanceVideoModelStandard,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeSeedance,
			UpstreamModelName: relaycommon.SeedanceVideoModelStandard,
		},
	}
	return context, recorder, info
}

func TestSeedanceRequestValidationAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _, info := newSeedanceContext(t, `{
		"model":"videos-standard",
		"prompt":"cinematic ocean",
		"duration":4,
		"resolution":" 4K ",
		"ratio":"16:9",
		"referenceImages":["https://example.com/image.png"]
	}`)

	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	requestBody, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	body, err := io.ReadAll(requestBody)
	require.NoError(t, err)

	var upstream relaycommon.TaskSubmitReq
	require.NoError(t, common.Unmarshal(body, &upstream))
	assert.Equal(t, relaycommon.SeedanceVideoModelStandard, upstream.Model)
	assert.Equal(t, 4, upstream.Duration)
	assert.Equal(t, "4k", upstream.Resolution)
	assert.Equal(t, "16:9", upstream.Ratio)
	assert.Equal(t, []string{"https://example.com/image.png"}, upstream.ReferenceImages)
	assert.Empty(t, upstream.Seconds)
}

func TestSeedanceRequestMapsPlaygroundImageToReferenceImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _, info := newSeedanceContext(t, `{
		"model":"videos-standard",
		"prompt":"animate this frame",
		"duration":4,
		"resolution":"720p",
		"image":"https://example.com/frame.png"
	}`)

	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	requestBody, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	body, err := io.ReadAll(requestBody)
	require.NoError(t, err)

	var upstream relaycommon.TaskSubmitReq
	require.NoError(t, common.Unmarshal(body, &upstream))
	assert.Equal(t, []string{"https://example.com/frame.png"}, upstream.ReferenceImages)
	assert.Empty(t, upstream.Image)
	assert.Empty(t, upstream.Images)
}

func TestSeedanceV2RequestUsesUpstreamProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _, info := newSeedanceContext(t, `{
		"model":"seedance2.0",
		"prompt":"animate this frame",
		"seconds":5,
		"size":"1280x720",
		"aspect_ratio":"16:9",
		"image":"https://example.com/frame.png"
	}`)
	info.OriginModelName = "seedance2.0"
	info.ChannelMeta.UpstreamModelName = "seedance2.0"

	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	requestBody, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	body, err := io.ReadAll(requestBody)
	require.NoError(t, err)

	var upstream map[string]any
	require.NoError(t, common.Unmarshal(body, &upstream))
	assert.Equal(t, "seedance2.0", upstream["model"])
	assert.Equal(t, "5", upstream["seconds"])
	assert.Equal(t, "1280x720", upstream["size"])
	assert.Equal(t, "16:9", upstream["aspect_ratio"])
	assert.Equal(t, "https://example.com/frame.png", upstream["image"])
	assert.NotContains(t, upstream, "duration")
	assert.NotContains(t, upstream, "resolution")
	assert.NotContains(t, upstream, "ratio")
	assert.NotContains(t, upstream, "referenceImages")
}

func TestSeedanceV2RequestValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
		code string
	}{
		{
			name: "seconds below minimum",
			body: `{"model":"seedance2.0","prompt":"animate","seconds":3,"size":"1280x720","aspect_ratio":"16:9"}`,
			code: "invalid_seconds",
		},
		{
			name: "seconds above maximum",
			body: `{"model":"seedance2.0","prompt":"animate","seconds":16,"size":"1280x720","aspect_ratio":"16:9"}`,
			code: "invalid_seconds",
		},
		{
			name: "mismatched size and ratio",
			body: `{"model":"seedance2.0","prompt":"animate","seconds":5,"size":"1280x720","aspect_ratio":"9:16"}`,
			code: "invalid_size",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _, info := newSeedanceContext(t, test.body)
			info.OriginModelName = "seedance2.0"
			info.ChannelMeta.UpstreamModelName = "seedance2.0"
			taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
}

func TestSeedanceRejectsRemix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _, info := newSeedanceContext(t, `{"model":"videos-standard","prompt":"change it","duration":4,"resolution":"720p"}`)
	info.Action = constant.TaskActionRemix
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
}

func TestSeedanceDoesNotAcceptXAIImageObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _, info := newSeedanceContext(t, `{
		"model":"videos-standard",
		"prompt":"animate",
		"duration":4,
		"resolution":"720p",
		"image":{"url":"https://example.com/frame.png"}
	}`)

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
}

func TestSeedanceParseTaskResult(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		status   model.TaskStatus
		progress string
		reason   string
	}{
		{name: "queued zero", body: `{"status":"queued","progress":0}`, status: model.TaskStatusQueued, progress: "0%"},
		{name: "in progress", body: `{"status":"in_progress","progress":53}`, status: model.TaskStatusInProgress, progress: "53%"},
		{name: "completed forces terminal", body: `{"status":"completed","progress":50}`, status: model.TaskStatusSuccess, progress: "100%"},
		{name: "failed forces terminal", body: `{"status":"failed","progress":8,"error":{"message":"blocked","code":"task_failed"}}`, status: model.TaskStatusFailure, progress: "100%", reason: "blocked"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(test.body))
			require.NoError(t, err)
			assert.Equal(t, string(test.status), result.Status)
			assert.Equal(t, test.progress, result.Progress)
			assert.Equal(t, test.reason, result.Reason)
		})
	}
}

func TestSeedanceParseTaskResultExtractsVideoURL(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "final metadata", body: `{"status":"completed","metadata":{"final_video_url":"https://oss.example/final.mp4"}}`, want: "https://oss.example/final.mp4"},
		{name: "root video", body: `{"status":"completed","video_url":"https://oss.example/video.mp4"}`, want: "https://oss.example/video.mp4"},
		{name: "root URL", body: `{"status":"completed","url":"https://oss.example/root.mp4"}`, want: "https://oss.example/root.mp4"},
		{name: "origin metadata", body: `{"status":"completed","metadata":{"origin_video_url":"https://oss.example/origin.mp4"}}`, want: "https://oss.example/origin.mp4"},
		{name: "metadata URL", body: `{"status":"completed","metadata":{"url":"https://oss.example/meta.mp4"}}`, want: "https://oss.example/meta.mp4"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(test.body))
			require.NoError(t, err)
			assert.Equal(t, test.want, result.Url)
		})
	}
}

func TestSeedanceDoResponseHidesUpstreamIdentityAndURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, recorder, info := newSeedanceContext(t, `{}`)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"id":"videos-standard_upstream",
			"task_id":"videos-standard_upstream",
			"model":"videos-standard",
			"status":"completed",
			"progress":50,
			"url":"https://upstream.example/video",
			"video_url":"https://upstream.example/video",
			"metadata":{"origin_video_url":"https://upstream.example/video"}
		}`)),
	}

	upstreamID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(context, response, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "videos-standard_upstream", upstreamID)
	assert.NotContains(t, string(taskData), "upstream.example")

	var publicResponse dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &publicResponse))
	assert.Equal(t, "task_public", publicResponse.ID)
	assert.Equal(t, "task_public", publicResponse.TaskID)
	assert.Equal(t, dto.VideoStatusCompleted, publicResponse.Status)
	assert.Equal(t, 100, publicResponse.Progress)
	assert.Empty(t, publicResponse.URL)
	assert.Empty(t, publicResponse.VideoURL)
	assert.Empty(t, publicResponse.Metadata)
}

func TestSeedanceConvertTaskUsesPlatformContentURL(t *testing.T) {
	previousBaseURL := system_setting.PublicApiBaseUrl
	system_setting.PublicApiBaseUrl = "https://api.frimodel.com"
	t.Cleanup(func() { system_setting.PublicApiBaseUrl = previousBaseURL })

	raw := []byte(`{
		"id":"vid_upstream",
		"task_id":"vid_upstream",
		"object":"video",
		"model":"videos-mini",
		"status":"completed",
		"progress":100,
		"url":"https://megavideos.oss-cn-hangzhou.aliyuncs.com/video.mp4?Signature=secret",
		"video_url":"https://megavideos.oss-cn-hangzhou.aliyuncs.com/video.mp4?Signature=secret",
		"metadata":{"final_video_url":"https://megavideos.oss-cn-hangzhou.aliyuncs.com/video.mp4?Signature=secret","cost_credits":70}
	}`)
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		Properties: model.Properties{OriginModelName: relaycommon.SeedanceVideoModelMini},
		Data:       raw,
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)

	require.NoError(t, err)
	assert.NotContains(t, string(body), "megavideos.oss-cn-hangzhou.aliyuncs.com")
	assert.NotContains(t, string(body), "Signature=secret")
	var response map[string]any
	require.NoError(t, common.Unmarshal(body, &response))
	proxyURL := "https://api.frimodel.com/v1/videos/task_public/content"
	assert.Equal(t, "task_public", response["id"])
	assert.Equal(t, "task_public", response["task_id"])
	assert.Equal(t, proxyURL, response["url"])
	assert.Equal(t, proxyURL, response["video_url"])
	metadata, ok := response["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, proxyURL, metadata["final_video_url"])
	assert.Equal(t, float64(70), metadata["cost_credits"])
}

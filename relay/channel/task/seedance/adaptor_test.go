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

func TestSeedanceRejectsRemix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _, info := newSeedanceContext(t, `{"model":"videos-standard","prompt":"change it","duration":4,"resolution":"720p"}`)
	info.Action = constant.TaskActionRemix
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

func TestSeedanceConvertCompletedTaskUsesOnlyPlatformURL(t *testing.T) {
	previousBaseURL := system_setting.PublicApiBaseUrl
	system_setting.PublicApiBaseUrl = "https://api.example.com"
	t.Cleanup(func() { system_setting.PublicApiBaseUrl = previousBaseURL })

	task := &model.Task{
		TaskID:      "task_public",
		Status:      model.TaskStatusSuccess,
		Progress:    "100%",
		SubmitTime:  100,
		FinishTime:  200,
		Properties:  model.Properties{OriginModelName: relaycommon.SeedanceVideoModelStandard},
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{Duration: 4}},
		Data:        []byte(`{"url":"https://upstream.example/video","metadata":{"url":"https://upstream.example/video"}}`),
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "upstream.example")
	assert.Contains(t, string(body), "https://api.example.com/v1/videos/task_public/content")
}

func TestSeedanceSanitizeTaskDataRemovesEveryUpstreamURL(t *testing.T) {
	body := (&TaskAdaptor{}).SanitizeTaskData([]byte(`{
		"model":"videos-standard",
		"status":"failed",
		"progress":8,
		"url":"https://upstream.example/content",
		"video_url":"https://upstream.example/content",
		"metadata":{"origin_video_url":"https://upstream.example/content"},
		"error":{"message":"download failed at https://upstream.example/content","code":"download_failed"}
	}`))

	assert.NotContains(t, string(body), "upstream.example")
	assert.NotContains(t, string(body), "origin_video_url")
	assert.Contains(t, string(body), "[upstream URL hidden]")
}

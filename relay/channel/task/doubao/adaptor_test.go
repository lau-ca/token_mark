package doubao

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestURLUsesChannelSpecificPath(t *testing.T) {
	tests := []struct {
		name        string
		channelType int
		want        string
	}{
		{name: "doubao video", channelType: constant.ChannelTypeDoubaoVideo, want: "https://example.com/v3/contents/generations/tasks"},
		{name: "volcengine", channelType: constant.ChannelTypeVolcEngine, want: "https://example.com/api/v3/contents/generations/tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &TaskAdaptor{}
			adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: tt.channelType, ChannelBaseUrl: "https://example.com/"}})
			got, err := adaptor.BuildRequestURL(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModelListIncludesAssetBillingCapability(t *testing.T) {
	assert.Contains(t, ModelList, constant.SeedanceAssetBillingModel)
}

func TestFetchTaskUsesChannelSpecificPath(t *testing.T) {
	for _, tt := range []struct {
		name        string
		channelType int
		wantPath    string
	}{
		{name: "doubao video", channelType: constant.ChannelTypeDoubaoVideo, wantPath: "/v3/contents/generations/tasks/upstream-1"},
		{name: "volcengine", channelType: constant.ChannelTypeVolcEngine, wantPath: "/api/v3/contents/generations/tasks/upstream-1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantPath, r.URL.Path)
				assert.Equal(t, "Bearer upstream-key", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"upstream-1"}`)
			}))
			defer server.Close()

			adaptor := &TaskAdaptor{ChannelType: tt.channelType}
			response, err := adaptor.FetchTask(server.URL, "upstream-key", map[string]any{"task_id": "upstream-1"}, "")
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
		})
	}
}

func TestConvertToRequestPayloadPreservesNativeContentAndExplicitValues(t *testing.T) {
	falseValue := false
	zero := 0
	duration := -1
	request := relaycommon.TaskSubmitReq{
		Model: "doubao-seedance-2-0-260128",
		Content: []relaycommon.TaskContentItem{
			{Type: "text", Text: "animate"},
			{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/video.mp4"}, Role: "reference_video"},
			{Type: "draft_task", DraftTask: &relaycommon.TaskDraftTask{ID: "draft-1"}},
		},
		ReturnLastFrame: &falseValue,
		GenerateAudio:   &falseValue,
		Priority:        &zero,
		Frames:          &zero,
		Watermark:       &falseValue,
		Duration:        duration,
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&request)

	require.NoError(t, err)
	require.Len(t, payload.Content, 3)
	assert.Equal(t, "https://example.com/video.mp4", payload.Content[1].VideoURL.URL)
	assert.Equal(t, "draft-1", payload.Content[2].DraftTask.ID)
	require.NotNil(t, payload.ReturnLastFrame)
	assert.False(t, *payload.ReturnLastFrame)
	require.NotNil(t, payload.Priority)
	assert.Zero(t, *payload.Priority)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, -1, *payload.Duration)
}

func TestConvertToRequestPayloadConvertsUnifiedReferences(t *testing.T) {
	request := relaycommon.TaskSubmitReq{
		Model:           "doubao-seedance-2-0-260128",
		Prompt:          "animate",
		Images:          []string{"https://example.com/image.png"},
		ReferenceVideos: []string{"https://example.com/video.mp4"},
		ReferenceAudios: []string{"https://example.com/audio.mp3"},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&request)

	require.NoError(t, err)
	require.Len(t, payload.Content, 4)
	assert.Equal(t, "image_url", payload.Content[0].Type)
	assert.Equal(t, "reference_video", payload.Content[1].Role)
	assert.Equal(t, "reference_audio", payload.Content[2].Role)
	assert.Equal(t, "text", payload.Content[3].Type)
}

func TestParseTaskResultTreatsCancelledAndExpiredAsTerminal(t *testing.T) {
	for _, status := range []string{"cancelled", "expired"} {
		t.Run(status, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"id":"task-1","status":"` + status + `"}`))
			require.NoError(t, err)
			assert.Equal(t, string(model.TaskStatusFailure), result.Status)
			assert.Equal(t, "100%", result.Progress)
			assert.Equal(t, status, result.Reason)
		})
	}
}

func TestParseTaskResultPreservesCompletionTokens(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"id":"task-1",
		"status":"succeeded",
		"content":{"video_url":"https://example.com/video.mp4"},
		"usage":{"completion_tokens":108000,"total_tokens":109000}
	}`))

	require.NoError(t, err)
	assert.Equal(t, 108000, result.CompletionTokens)
	assert.Equal(t, 109000, result.TotalTokens)
}

func TestConvertToNativeVideoPreservesDocumentedFields(t *testing.T) {
	task := &model.Task{
		TaskID: "upstream-1",
		Status: model.TaskStatusSuccess,
		Properties: model.Properties{
			OriginModelName: "doubao-seedance-2-0-260128",
		},
		Data: []byte(`{
			"id":"hidden-id",
			"model":"Doubao-Seedance-2.0",
			"status":"succeeded",
			"content":{"video_url":"https://example.com/video.mp4","last_frame_url":"https://example.com/frame.jpg"},
			"priority":0,
			"draft":false,
			"generate_audio":true,
			"execution_expires_after":172800,
			"usage":{"completion_tokens":108000,"total_tokens":108000}
		}`),
	}

	body, err := (&TaskAdaptor{}).ConvertToNativeVideo(task)

	require.NoError(t, err)
	var response map[string]any
	require.NoError(t, common.Unmarshal(body, &response))
	assert.Equal(t, "upstream-1", response["id"])
	content := response["content"].(map[string]any)
	assert.Equal(t, "https://example.com/frame.jpg", content["last_frame_url"])
	assert.Equal(t, true, response["generate_audio"])
}

func TestDoResponseReturnsNativeCreateShapeForV3(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v3/contents/generations/tasks", strings.NewReader(""))
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"upstream-1"}`)),
	}

	taskID, _, taskErr := (&TaskAdaptor{}).DoResponse(context, response, &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	})

	require.Nil(t, taskErr)
	assert.Equal(t, "upstream-1", taskID)
	assert.JSONEq(t, `{"id":"upstream-1"}`, recorder.Body.String())
}

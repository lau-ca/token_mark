package relay

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyVideoResponsePrivacyDisabledReturnsOriginalBytes(t *testing.T) {
	raw := []byte("{ \"id\": \"upstream-id\", \"metadata\": {\"url\": \"https://upstream.example/video.mp4?token=secret\"}, \"unknown\": true }")
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	channel := newPrivacyTestChannel(t, constant.ChannelTypeSora, false)

	got, err := ApplyVideoResponsePrivacy(raw, task, channel)

	require.NoError(t, err)
	assert.Equal(t, raw, got)
}

func TestApplyVideoResponsePrivacyUnsupportedChannelReturnsOriginalBytes(t *testing.T) {
	raw := []byte("{\n  \"id\": \"upstream-id\",\n  \"url\": \"https://upstream.example/video.mp4?token=secret\"\n}")
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	channel := newPrivacyTestChannel(t, constant.ChannelTypeVertexAi, true)

	got, err := ApplyVideoResponsePrivacy(raw, task, channel)

	require.NoError(t, err)
	assert.Equal(t, raw, got)
}

func TestVideoResponsePrivacySkipsUnsupportedPlatforms(t *testing.T) {
	tests := []struct {
		name string
		task *model.Task
		want bool
	}{
		{name: "nil task", task: nil, want: false},
		{name: "empty platform fails closed", task: &model.Task{}, want: true},
		{name: "openai", task: &model.Task{Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeOpenAI))}, want: true},
		{name: "sora", task: &model.Task{Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeSora))}, want: true},
		{name: "seedance", task: &model.Task{Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeSeedance))}, want: true},
		{name: "vertex", task: &model.Task{Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeVertexAi))}, want: false},
		{name: "named legacy platform", task: &model.Task{Platform: constant.TaskPlatformSuno}, want: false},
		{name: "invalid numeric platform", task: &model.Task{Platform: constant.TaskPlatform("not-a-channel")}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldResolveVideoPrivacyChannel(tt.task))
		})
	}

	unsupportedChannel := &model.Channel{
		Type:          constant.ChannelTypeVertexAi,
		OtherSettings: "malformed settings must remain untouched",
	}
	enabled, err := resolveVideoResponsePrivacy(unsupportedChannel)
	require.NoError(t, err)
	assert.False(t, enabled)
	assert.Equal(t, "malformed settings must remain untouched", unsupportedChannel.OtherSettings)

	supportedChannel := &model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		OtherSettings: "malformed settings must fail closed",
	}
	enabled, err = resolveVideoResponsePrivacy(supportedChannel)
	require.Error(t, err)
	assert.False(t, enabled)
	assert.Equal(t, "malformed settings must fail closed", supportedChannel.OtherSettings)
}

func TestSeedanceChannelAlwaysUsesPlatformVideoURL(t *testing.T) {
	channel := newPrivacyTestChannel(t, constant.ChannelTypeSeedance, false)
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	response := []byte(`{"id":"upstream-id","url":"https://upstream.example/video","video_url":"https://upstream.example/video","metadata":{"origin_video_url":"https://upstream.example/video"}}`)

	result, err := ApplyVideoResponsePrivacy(response, task, channel)
	require.NoError(t, err)
	assert.NotContains(t, string(result), "upstream.example")
	assert.Contains(t, string(result), task.TaskID)
}

func TestApplyVideoResponsePrivacyMalformedResponse(t *testing.T) {
	raw := []byte(`{"id":"upstream-id","metadata":`)
	task := newPrivacyTestTask(model.TaskStatusSuccess)

	t.Run("enabled returns error", func(t *testing.T) {
		channel := newPrivacyTestChannel(t, constant.ChannelTypeOpenAI, true)

		got, err := ApplyVideoResponsePrivacy(raw, task, channel)

		require.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("disabled returns original bytes", func(t *testing.T) {
		channel := newPrivacyTestChannel(t, constant.ChannelTypeOpenAI, false)

		got, err := ApplyVideoResponsePrivacy(raw, task, channel)

		require.NoError(t, err)
		assert.Equal(t, raw, got)
	})
}

func TestApplyVideoResponsePrivacyCanonicalizesEnabledResponse(t *testing.T) {
	setPrivacyTestBaseURL(t)
	raw := []byte(`{
		"id":"upstream-id",
		"task_id":"upstream-task-id",
		"object":"video",
		"model":"upstream-model",
		"status":"completed",
		"progress":99,
		"seconds":"15",
		"size":"1280x720",
		"url":"https://cdn.example/root.mp4?signature=secret",
		"video_url":"https://cdn.example/video-url.mp4?signature=secret",
		"created_at":101,
		"metadata":{"url":"https://cdn.example/video.mp4?signature=secret","nested":{"url":"https://cdn.example/nested"}},
		"download_url":"https://cdn.example/download?signature=secret",
		"response":{"video":{"url":"https://cdn.example/raw?signature=secret"}}
	}`)
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	task.Progress = "67%"
	channel := newPrivacyTestChannel(t, constant.ChannelTypeSora, true)

	got, err := ApplyVideoResponsePrivacy(raw, task, channel)

	require.NoError(t, err)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(got, &video))
	assert.Equal(t, task.TaskID, video.ID)
	assert.Equal(t, task.TaskID, video.TaskID)
	assert.Equal(t, task.Properties.OriginModelName, video.Model)
	assert.Equal(t, dto.VideoStatusCompleted, video.Status)
	assert.Equal(t, 67, video.Progress)
	assert.Equal(t, "15", video.Seconds)
	assert.Equal(t, "1280x720", video.Size)
	assert.Equal(t, int64(101), video.CreatedAt)
	proxyURL := taskcommon.BuildProxyURL(task.TaskID)
	assert.Equal(t, proxyURL, video.URL)
	assert.Equal(t, proxyURL, video.VideoURL)
	assert.Equal(t, map[string]any{"url": proxyURL}, video.Metadata)
	assert.NotContains(t, string(got), "upstream-id")
	assert.NotContains(t, string(got), "upstream-task-id")
	assert.NotContains(t, string(got), "cdn.example")
	assert.NotContains(t, string(got), "signature=secret")
	assert.NotContains(t, string(got), "download_url")
	assert.NotContains(t, string(got), "response")
}

func TestApplyVideoResponsePrivacyRemovesDataAndNestedVideoURLs(t *testing.T) {
	setPrivacyTestBaseURL(t)
	raw := []byte(`{
		"id":"upstream-id",
		"metadata":{"url":"data:video/mp4;base64,PRIVATE_VIDEO_DATA"},
		"url":"data:video/mp4;base64,PRIVATE_ROOT_DATA",
		"response":{"videos":[{"bytesBase64Encoded":"PRIVATE_VERTEX_DATA"}]}
	}`)
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	channel := newPrivacyTestChannel(t, constant.ChannelTypeOpenAI, true)

	got, err := ApplyVideoResponsePrivacy(raw, task, channel)

	require.NoError(t, err)
	assert.NotContains(t, string(got), "PRIVATE_VIDEO_DATA")
	assert.NotContains(t, string(got), "PRIVATE_ROOT_DATA")
	assert.NotContains(t, string(got), "PRIVATE_VERTEX_DATA")
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(got, &video))
	assert.Equal(t, map[string]any{"url": taskcommon.BuildProxyURL(task.TaskID)}, video.Metadata)
}

func TestApplyVideoResponsePrivacyNonSuccessHasNoResultURL(t *testing.T) {
	setPrivacyTestBaseURL(t)
	raw := []byte(`{
		"id":"upstream-id",
		"metadata":{"url":"https://cdn.example/video.mp4?signature=secret"},
		"error":{"message":"generation failed: https://cdn.example/error?signature=secret","code":"provider_error"}
	}`)
	task := newPrivacyTestTask(model.TaskStatusFailure)
	task.Progress = "100%"
	channel := newPrivacyTestChannel(t, constant.ChannelTypeSora, true)

	got, err := ApplyVideoResponsePrivacy(raw, task, channel)

	require.NoError(t, err)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(got, &video))
	assert.Equal(t, dto.VideoStatusFailed, video.Status)
	assert.Equal(t, 100, video.Progress)
	assert.Empty(t, video.URL)
	assert.Empty(t, video.VideoURL)
	assert.Nil(t, video.Metadata)
	require.NotNil(t, video.Error)
	assert.Equal(t, "generation failed: [video URL hidden]", video.Error.Message)
	assert.Equal(t, "provider_error", video.Error.Code)
	assert.NotContains(t, string(got), "cdn.example")
}

func TestTaskModel2UserDtoSanitizesResultAndData(t *testing.T) {
	setPrivacyTestBaseURL(t)
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	task.FailReason = "https://legacy.example/video.mp4?token=legacy-secret"
	task.PrivateData.ResultURL = "https://private.example/video.mp4?token=private-secret"
	task.Data = []byte(`{
		"id":"upstream-id",
		"task_id":"upstream-task-id",
		"seconds":"15",
		"size":"1280x720",
		"metadata":{"url":"https://private.example/video.mp4?token=private-secret"},
		"raw":{"url":"https://nested.example/video.mp4?token=nested-secret"}
	}`)
	originalData := append([]byte(nil), task.Data...)
	channel := newPrivacyTestChannel(t, constant.ChannelTypeOpenAI, true)

	got := TaskModel2UserDto(task, channel)

	proxyURL := taskcommon.BuildProxyURL(task.TaskID)
	assert.Equal(t, proxyURL, got.ResultURL)
	assert.Empty(t, got.FailReason)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(got.Data, &video))
	assert.Equal(t, task.TaskID, video.ID)
	assert.Equal(t, task.TaskID, video.TaskID)
	assert.Equal(t, task.Properties.OriginModelName, video.Model)
	assert.Equal(t, dto.VideoStatusCompleted, video.Status)
	assert.Equal(t, proxyURL, video.URL)
	assert.Equal(t, proxyURL, video.VideoURL)
	assert.Equal(t, map[string]any{"url": proxyURL}, video.Metadata)
	assert.Equal(t, "15", video.Seconds)
	assert.Equal(t, "1280x720", video.Size)
	assert.NotContains(t, string(got.Data), "private.example")
	assert.NotContains(t, string(got.Data), "nested.example")
	assert.Equal(t, "https://private.example/video.mp4?token=private-secret", task.PrivateData.ResultURL)
	assert.Equal(t, "https://legacy.example/video.mp4?token=legacy-secret", task.FailReason)
	assert.Equal(t, originalData, []byte(task.Data))
}

func TestTaskModel2UserDtoFailsClosedWithoutChannel(t *testing.T) {
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	task.FailReason = "https://legacy.example/video.mp4?token=secret"
	task.PrivateData.ResultURL = "https://private.example/video.mp4?token=secret"
	task.Data = []byte(`{"metadata":{"url":"https://private.example/video.mp4?token=secret"},"url":"https://nested.example/video.mp4"}`)

	got := TaskModel2UserDto(task, nil)

	assert.Empty(t, got.ResultURL)
	assert.Empty(t, got.FailReason)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(got.Data, &video))
	assert.Nil(t, video.Metadata)
	assert.NotContains(t, string(got.Data), "private.example")
	assert.NotContains(t, string(got.Data), "nested.example")
}

func TestTaskModel2UserDtoFailsClosedOnMalformedSupportedSettings(t *testing.T) {
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	task.FailReason = "https://legacy.example/video.mp4?token=secret"
	task.PrivateData.ResultURL = "https://private.example/video.mp4?token=secret"
	task.Data = []byte(`{"metadata":{"url":"https://private.example/video.mp4?token=secret"},"url":"https://nested.example/video.mp4"}`)
	channel := &model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		OtherSettings: "malformed settings must remain untouched",
	}

	got := TaskModel2UserDto(task, channel)

	assert.Empty(t, got.ResultURL)
	assert.Empty(t, got.FailReason)
	assert.NotContains(t, string(got.Data), "private.example")
	assert.NotContains(t, string(got.Data), "nested.example")
	assert.Equal(t, "malformed settings must remain untouched", channel.OtherSettings)
}

func TestTaskModel2UserDtoClearsFailedLegacyResultURL(t *testing.T) {
	task := newPrivacyTestTask(model.TaskStatusFailure)
	task.FailReason = "https://legacy.example/video.mp4?token=secret"
	task.Data = []byte(`{"error":{"message":"generation failed","code":"provider_error"},"url":"https://legacy.example/video.mp4?token=secret"}`)
	channel := newPrivacyTestChannel(t, constant.ChannelTypeSora, true)

	got := TaskModel2UserDto(task, channel)

	assert.Empty(t, got.ResultURL)
	assert.Empty(t, got.FailReason)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(got.Data, &video))
	assert.Empty(t, video.URL)
	assert.Empty(t, video.VideoURL)
	assert.Nil(t, video.Metadata)
	require.NotNil(t, video.Error)
	assert.Equal(t, "generation failed", video.Error.Message)
}

func TestTaskModel2UserDtoRedactsURLInsideFailureText(t *testing.T) {
	task := newPrivacyTestTask(model.TaskStatusFailure)
	task.FailReason = "generation failed: https://legacy.example/video.mp4?token=secret"
	task.Data = []byte(`{"error":{"message":"provider failed at https://private.example/video.mp4?token=secret","code":"provider_error"}}`)
	channel := newPrivacyTestChannel(t, constant.ChannelTypeSora, true)

	got := TaskModel2UserDto(task, channel)

	assert.Equal(t, "generation failed: [video URL hidden]", got.FailReason)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(got.Data, &video))
	require.NotNil(t, video.Error)
	assert.Equal(t, "provider failed at [video URL hidden]", video.Error.Message)
	assert.NotContains(t, string(got.Data), "private.example")
}

func TestTaskModel2DtoKeepsAdminDataRaw(t *testing.T) {
	task := newPrivacyTestTask(model.TaskStatusSuccess)
	task.FailReason = "https://legacy.example/video.mp4?token=secret"
	task.PrivateData.ResultURL = "https://private.example/video.mp4?token=secret"
	task.Data = []byte(`{"metadata":{"url":"https://private.example/video.mp4?token=secret"},"upstream_task_id":"private-task-id"}`)

	got := TaskModel2Dto(task)

	assert.Equal(t, task.PrivateData.ResultURL, got.ResultURL)
	assert.Equal(t, task.FailReason, got.FailReason)
	assert.Equal(t, []byte(task.Data), []byte(got.Data))
}

func newPrivacyTestTask(status model.TaskStatus) *model.Task {
	return &model.Task{
		TaskID:    "task_public_123",
		Status:    status,
		Progress:  "42%",
		ChannelId: 10,
		Properties: model.Properties{
			OriginModelName: "videos-standard",
		},
	}
}

func newPrivacyTestChannel(t *testing.T, channelType int, enabled bool) *model.Channel {
	t.Helper()
	settings, err := common.Marshal(dto.ChannelOtherSettings{ReplaceVideoURLsWithProxy: enabled})
	require.NoError(t, err)
	return &model.Channel{
		Type:          channelType,
		OtherSettings: string(settings),
	}
}

func setPrivacyTestBaseURL(t *testing.T) {
	t.Helper()
	originalPublicBaseURL := system_setting.PublicApiBaseUrl
	originalServerAddress := system_setting.ServerAddress
	system_setting.PublicApiBaseUrl = "https://gateway.example"
	system_setting.ServerAddress = "http://localhost:3000"
	t.Cleanup(func() {
		system_setting.PublicApiBaseUrl = originalPublicBaseURL
		system_setting.ServerAddress = originalServerAddress
	})
}

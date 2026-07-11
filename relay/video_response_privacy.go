package relay

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
)

var videoURLInTextPattern = regexp.MustCompile(`(?i)(?:(?:https?|s3|gs|oss|tos)://|//|data:)[^\s"'<>]+`)

func resolveVideoResponsePrivacy(channel *model.Channel) (bool, error) {
	if channel == nil || !dto.SupportsVideoURLProxyReplacement(channel.Type) {
		return false, nil
	}
	settings, err := channel.ParseOtherSettings()
	if err != nil {
		return false, err
	}
	return settings.ShouldReplaceVideoURLs(channel.Type), nil
}

func ShouldResolveVideoPrivacyChannel(task *model.Task) bool {
	if task == nil {
		return false
	}
	if task.Platform == "" {
		return true
	}
	channelType, err := strconv.Atoi(string(task.Platform))
	return err == nil && dto.SupportsVideoURLProxyReplacement(channelType)
}

func ApplyVideoResponsePrivacy(response []byte, task *model.Task, channel *model.Channel) ([]byte, error) {
	enabled, err := resolveVideoResponsePrivacy(channel)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return response, nil
	}
	return canonicalizeOpenAIVideoResponse(response, task, taskcommon.BuildProxyURL(task.TaskID))
}

func canonicalizeOpenAIVideoResponse(response []byte, task *model.Task, proxyURL string) ([]byte, error) {
	video := dto.NewOpenAIVideo()
	if err := common.Unmarshal(response, video); err != nil {
		return nil, err
	}
	applyOpenAIVideoPrivacy(video, task, proxyURL)
	return common.Marshal(video)
}

func applyOpenAIVideoPrivacy(video *dto.OpenAIVideo, task *model.Task, proxyURL string) {
	video.ID = task.TaskID
	video.TaskID = task.TaskID
	video.Model = task.Properties.OriginModelName
	video.Status = task.Status.ToVideoStatus()
	video.SetProgressStr(task.Progress)
	video.URL = ""
	video.VideoURL = ""
	video.ReplaceMetadataURL("")
	if video.Error != nil {
		video.Error.Message = sanitizeVideoUserText(video.Error.Message)
		video.Error.Code = sanitizeVideoUserText(video.Error.Code)
	}
	if task.Status == model.TaskStatusSuccess && proxyURL != "" {
		video.URL = proxyURL
		video.VideoURL = proxyURL
		video.ReplaceMetadataURL(proxyURL)
	}
}

func TaskModel2UserDto(task *model.Task, channel *model.Channel) *dto.TaskDto {
	taskDto := TaskModel2Dto(task)
	enabled, settingsErr := resolveVideoResponsePrivacy(channel)
	if channel != nil && settingsErr == nil && !enabled {
		return taskDto
	}

	proxyURL := ""
	if settingsErr == nil && enabled {
		proxyURL = taskcommon.BuildProxyURL(task.TaskID)
	}

	video := dto.NewOpenAIVideo()
	if err := common.Unmarshal(task.Data, video); err != nil {
		video = dto.NewOpenAIVideo()
	}
	applyOpenAIVideoPrivacy(video, task, proxyURL)
	taskDto.Data, _ = common.Marshal(video)
	taskDto.ResultURL = ""
	taskDto.FailReason = sanitizeVideoUserText(taskDto.FailReason)
	if task.Status == model.TaskStatusSuccess {
		taskDto.FailReason = ""
		if proxyURL != "" {
			taskDto.ResultURL = proxyURL
		}
	}
	return taskDto
}

func sanitizeVideoUserText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || isVideoResultURL(value) {
		return ""
	}
	return videoURLInTextPattern.ReplaceAllString(value, "[video URL hidden]")
}

func isVideoResultURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "s3://") ||
		strings.HasPrefix(value, "gs://") ||
		strings.HasPrefix(value, "oss://") ||
		strings.HasPrefix(value, "tos://") ||
		strings.HasPrefix(value, "data:") ||
		strings.HasPrefix(value, "//")
}

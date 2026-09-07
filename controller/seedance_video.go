package controller

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxSeedanceTaskPageSize = 100

type seedanceTaskDeleter interface {
	DeleteTask(baseURL, key, taskID, proxy string) (*http.Response, error)
}

func SeedanceTaskFetch(c *gin.Context) {
	task, ok := getOwnedSeedanceTask(c)
	if !ok {
		return
	}
	body, err := convertSeedanceTask(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "server_error"}})
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}

func SeedanceTaskList(c *gin.Context) {
	pageNum, err := parsePositiveQueryInt(c.Query("page_num"), 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "page_num must be a positive integer", "type": "invalid_request_error"}})
		return
	}
	pageSize, err := parsePositiveQueryInt(c.Query("page_size"), 20)
	if err != nil || pageSize > maxSeedanceTaskPageSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("page_size must be between 1 and %d", maxSeedanceTaskPageSize), "type": "invalid_request_error"}})
		return
	}

	tasks, err := model.GetUserTasksByPlatform(c.GetInt("id"), seedanceTaskPlatform())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to list tasks", "type": "server_error"}})
		return
	}

	taskIDs := make(map[string]struct{})
	for _, taskID := range strings.Split(c.Query("filter.task_ids"), ",") {
		if taskID = strings.TrimSpace(taskID); taskID != "" {
			taskIDs[taskID] = struct{}{}
		}
	}
	statusFilter := normalizeSeedanceStatusFilter(c.Query("filter.status"))
	modelFilter := strings.TrimSpace(c.Query("filter.model"))
	serviceTierFilter := strings.TrimSpace(c.Query("filter.service_tier"))
	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		if len(taskIDs) > 0 {
			if _, exists := taskIDs[task.TaskID]; !exists {
				continue
			}
		}
		body, err := convertSeedanceTask(task)
		if err != nil {
			continue
		}
		var item map[string]any
		if err := common.Unmarshal(body, &item); err != nil {
			continue
		}
		if statusFilter != "" && item["status"] != statusFilter {
			continue
		}
		if modelFilter != "" && item["model"] != modelFilter && task.Properties.OriginModelName != modelFilter {
			continue
		}
		if serviceTierFilter != "" && item["service_tier"] != serviceTierFilter {
			continue
		}
		items = append(items, item)
	}

	total := len(items)
	start := (pageNum - 1) * pageSize
	if start >= total {
		items = []map[string]any{}
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = items[start:end]
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": items})
}

func SeedanceTaskDelete(c *gin.Context) {
	task, ok := getOwnedSeedanceTask(c)
	if !ok {
		return
	}
	channelModel, err := model.GetChannelById(task.ChannelId, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to load task channel", "type": "server_error"}})
		return
	}
	if channelModel.Type != constant.ChannelTypeDoubaoVideo {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "task is not a Doubao Video task", "type": "invalid_request_error"}})
		return
	}
	key, _, keyErr := channelModel.GetNextEnabledKey()
	if keyErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "task channel has no available key", "type": "server_error"}})
		return
	}
	baseURL := channelModel.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[channelModel.Type]
	}
	adaptor := relay.GetTaskAdaptor(task.Platform)
	if adaptor == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "task adaptor is unavailable", "type": "server_error"}})
		return
	}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:    channelModel.Type,
		ChannelBaseUrl: baseURL,
		ApiKey:         key,
	}})
	deleter, ok := adaptor.(seedanceTaskDeleter)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{"message": "task cancellation is not supported", "type": "server_error"}})
		return
	}
	response, err := deleter.DeleteTask(baseURL, key, task.GetUpstreamTaskID(), channelModel.GetSetting().Proxy)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "upstream task cancellation failed", "type": "server_error"}})
		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "failed to read upstream response", "type": "server_error"}})
		return
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		c.Data(response.StatusCode, "application/json", responseBody)
		return
	}

	taskResult, err := adaptor.ParseTaskResult(task, response, responseBody)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "invalid upstream task response", "type": "server_error"}})
		return
	}
	previousStatus := task.Status
	task.Data = responseBody
	if taskResult.Status != "" {
		task.Status = model.TaskStatus(taskResult.Status)
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}
	task.FailReason = taskResult.Reason
	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		task.FinishTime = time.Now().Unix()
	}
	won, err := task.UpdateWithStatus(previousStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to update task", "type": "server_error"}})
		return
	}
	if won && previousStatus != model.TaskStatusFailure && previousStatus != model.TaskStatusSuccess && task.Status == model.TaskStatusFailure && task.Quota != 0 {
		service.RefundTaskQuota(c.Request.Context(), task, task.FailReason)
	}
	if !won {
		reloaded, exists, reloadErr := model.GetByTaskId(c.GetInt("id"), task.TaskID)
		if reloadErr != nil || !exists {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": "task state changed concurrently", "type": "server_error"}})
			return
		}
		task = reloaded
	}
	body, err := convertSeedanceTask(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "server_error"}})
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}

func getOwnedSeedanceTask(c *gin.Context) (*model.Task, bool) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), c.Param("task_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to load task", "type": "server_error"}})
		return nil, false
	}
	if !exists || task.Platform != seedanceTaskPlatform() {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "task not found", "type": "invalid_request_error"}})
		return nil, false
	}
	return task, true
}

func convertSeedanceTask(task *model.Task) ([]byte, error) {
	adaptor := relay.GetTaskAdaptor(task.Platform)
	if adaptor == nil {
		return nil, fmt.Errorf("task adaptor is unavailable")
	}
	converter, ok := adaptor.(channel.NativeVideoConverter)
	if !ok {
		return nil, fmt.Errorf("native task conversion is unavailable")
	}
	return converter.ConvertToNativeVideo(task)
}

func seedanceTaskPlatform() constant.TaskPlatform {
	return constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo))
}

func parsePositiveQueryInt(value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("value must be a positive integer")
	}
	return parsed, nil
}

func normalizeSeedanceStatusFilter(status string) string {
	switch strings.TrimSpace(status) {
	case "pending":
		return "queued"
	case "processing", "in_progress":
		return "running"
	case "completed":
		return "succeeded"
	default:
		return strings.TrimSpace(status)
	}
}

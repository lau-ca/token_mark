package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSeedanceVideoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	service.InitHttpClient()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		service.InitHttpClient()
	})
	return db
}

func seedNativeSeedanceTask(t *testing.T, db *gorm.DB, taskID string, userID int, status string) *model.Task {
	t.Helper()
	task := &model.Task{
		TaskID:    taskID,
		UserId:    userID,
		ChannelId: 1,
		Platform:  seedanceTaskPlatform(),
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		Properties: model.Properties{
			OriginModelName: "doubao-seedance-2-0-260128",
		},
		PrivateData: model.TaskPrivateData{UpstreamTaskID: taskID},
		Data: json.RawMessage(`{
			"id":"` + taskID + `",
			"model":"doubao-seedance-2-0-260128",
			"status":"` + status + `",
			"content":{"video_url":"https://example.com/video.mp4","last_frame_url":"https://example.com/frame.jpg"},
			"service_tier":"default"
		}`),
	}
	require.NoError(t, db.Create(task).Error)
	return task
}

func TestSeedanceTaskFetchReturnsNativeObjectAndEnforcesOwnership(t *testing.T) {
	db := setupSeedanceVideoTestDB(t)
	seedNativeSeedanceTask(t, db, "upstream-1", 101, "succeeded")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/v3/contents/generations/tasks/upstream-1", nil)
	context.Set("id", 101)
	context.Params = gin.Params{{Key: "task_id", Value: "upstream-1"}}

	SeedanceTaskFetch(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "upstream-1", response["id"])
	assert.NotContains(t, response, "code")

	otherRecorder := httptest.NewRecorder()
	otherContext, _ := gin.CreateTestContext(otherRecorder)
	otherContext.Request = httptest.NewRequest(http.MethodGet, "/v3/contents/generations/tasks/upstream-1", nil)
	otherContext.Set("id", 202)
	otherContext.Params = gin.Params{{Key: "task_id", Value: "upstream-1"}}
	SeedanceTaskFetch(otherContext)
	assert.Equal(t, http.StatusNotFound, otherRecorder.Code)
}

func TestSeedanceTaskListFiltersAndPaginatesNativeTasks(t *testing.T) {
	db := setupSeedanceVideoTestDB(t)
	seedNativeSeedanceTask(t, db, "upstream-1", 101, "succeeded")
	seedNativeSeedanceTask(t, db, "upstream-2", 101, "failed")
	seedNativeSeedanceTask(t, db, "other-user", 202, "succeeded")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/v3/contents/generations/tasks?page_num=1&page_size=10&filter.status=succeeded&filter.service_tier=default", nil)
	context.Set("id", 101)

	SeedanceTaskList(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Total int              `json:"total"`
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, 1, response.Total)
	require.Len(t, response.Items, 1)
	assert.Equal(t, "upstream-1", response.Items[0]["id"])
}

func TestSeedanceTaskDeleteForwardsUpstreamIDAndUpdatesTask(t *testing.T) {
	db := setupSeedanceVideoTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v3/contents/generations/tasks/upstream-1", r.URL.Path)
		assert.Equal(t, "Bearer upstream-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"upstream-1","model":"doubao-seedance-2-0-260128","status":"cancelled","content":null,"error":null}`)
	}))
	defer server.Close()

	channelModel := &model.Channel{
		Id:      1,
		Type:    constant.ChannelTypeDoubaoVideo,
		Key:     "upstream-key",
		Status:  common.ChannelStatusEnabled,
		Name:    "onfishes",
		BaseURL: common.GetPointer(server.URL),
		Models:  "doubao-seedance-2-0-260128",
		Group:   "default",
	}
	require.NoError(t, db.Create(channelModel).Error)
	task := seedNativeSeedanceTask(t, db, "upstream-1", 101, "running")
	task.Status = model.TaskStatusInProgress
	require.NoError(t, db.Model(task).Update("status", task.Status).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/v3/contents/generations/tasks/upstream-1", strings.NewReader(""))
	context.Set("id", 101)
	context.Params = gin.Params{{Key: "task_id", Value: "upstream-1"}}

	SeedanceTaskDelete(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"cancelled"`)
	var reloaded model.Task
	require.NoError(t, db.First(&reloaded, task.ID).Error)
	assert.Equal(t, string(model.TaskStatusFailure), string(reloaded.Status))
	assert.Equal(t, "100%", reloaded.Progress)
}

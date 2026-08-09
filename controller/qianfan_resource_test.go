package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newQianfanResourceTestContext(target string) *gin.Context {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return context
}

func TestBuildQianfanResourceRequestElementPagination(t *testing.T) {
	t.Parallel()

	context := newQianfanResourceTestContext("/api/channel/1/qianfan/elements?model=Presets-Elements&pageNum=4&pageSize=500")
	request, err := buildQianfanResourceRequest(context, &model.Channel{
		Type:    constant.ChannelTypeBaiduV2,
		BaseURL: new(string),
	}, "test-key", qianfanListElements)
	require.NoError(t, err)

	query := request.URL.Query()
	assert.Equal(t, "Presets-Elements", query.Get("model"))
	assert.Equal(t, "4", query.Get("pageNum"))
	assert.Equal(t, "500", query.Get("pageSize"))
}

func TestBuildQianfanResourceRequestUsesOfficialVoiceModels(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{Type: constant.ChannelTypeBaiduV2, BaseURL: new(string)}

	listRequest, err := buildQianfanResourceRequest(newQianfanResourceTestContext("/api/channel/1/qianfan/voices"), channel, "test-key", qianfanListVoices)
	require.NoError(t, err)
	assert.Equal(t, "/beta/video/generations/qianfan-video/list", listRequest.URL.Path)
	assert.Equal(t, "Custom-Voice", listRequest.URL.Query().Get("model"))
	assert.Equal(t, "1", listRequest.URL.Query().Get("pageNum"))
	assert.Equal(t, "30", listRequest.URL.Query().Get("pageSize"))

	taskContext := newQianfanResourceTestContext("/api/channel/1/qianfan/voices/tasks/voice-task")
	taskContext.Params = gin.Params{{Key: "task_id", Value: "voice-task"}}
	taskRequest, err := buildQianfanResourceRequest(taskContext, channel, "test-key", qianfanGetVoiceTask)
	require.NoError(t, err)
	assert.Equal(t, "Custom-Voice", taskRequest.URL.Query().Get("model"))
}

func TestBuildQianfanResourceRequestRejectsOutOfRangePagination(t *testing.T) {
	t.Parallel()

	context := newQianfanResourceTestContext("/api/channel/1/qianfan/elements?pageSize=501")
	_, err := buildQianfanResourceRequest(context, &model.Channel{
		Type:    constant.ChannelTypeBaiduV2,
		BaseURL: new(string),
	}, "test-key", qianfanListElements)
	require.Error(t, err)
}

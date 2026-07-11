package sora

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func estimateBillingRatios(t *testing.T, req relaycommon.TaskSubmitReq) map[string]float64 {
	t.Helper()
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", req)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	return (&TaskAdaptor{}).EstimateBilling(context, info)
}

func buildValidatedRequestBody(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "upstream-video-model",
		},
	}
	require.Nil(t, relaycommon.ValidateMultipartDirect(context, info))

	requestBody, err := (&TaskAdaptor{}).BuildRequestBody(context, info)
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(requestBody)
	require.NoError(t, err)
	var bodyMap map[string]interface{}
	require.NoError(t, appcommon.Unmarshal(bodyBytes, &bodyMap))
	return bodyMap
}

func TestBuildRequestBodyCanonicalizesSeedanceVideoParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bodyMap := buildValidatedRequestBody(t, `{
		"model":"videos-standard",
		"prompt":"make a short film",
		"duration":4,
		"seconds":"4",
		"resolution":" 4K ",
		"ratio":"16:9",
		"referenceImages":["https://example.com/image.png"]
	}`)

	assert.Equal(t, "upstream-video-model", bodyMap["model"])
	assert.Equal(t, float64(4), bodyMap["duration"])
	assert.Equal(t, "4k", bodyMap["resolution"])
	assert.NotContains(t, bodyMap, "seconds")
	assert.Equal(t, "16:9", bodyMap["ratio"])
	assert.Contains(t, bodyMap, "referenceImages")
}

func TestBuildRequestBodyKeepsLegacyVideoParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bodyMap := buildValidatedRequestBody(t, `{
		"model":"sora-2",
		"prompt":"make a short film",
		"duration":4.5,
		"seconds":"four",
		"resolution":" 4K "
	}`)

	assert.Equal(t, "upstream-video-model", bodyMap["model"])
	assert.Equal(t, 4.5, bodyMap["duration"])
	assert.Equal(t, "four", bodyMap["seconds"])
	assert.Equal(t, " 4K ", bodyMap["resolution"])
}

func TestEstimateBillingSeedanceVideoDoesNotReturnPricingRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, modelName := range []string{
		relaycommon.SeedanceVideoModelFast,
		relaycommon.SeedanceVideoModelMini,
		relaycommon.SeedanceVideoModelStandard,
	} {
		t.Run(modelName, func(t *testing.T) {
			ratios := estimateBillingRatios(t, relaycommon.TaskSubmitReq{
				Model:      modelName,
				Duration:   15,
				Resolution: "720p",
			})
			assert.Empty(t, ratios)
		})
	}
}

func TestValidateRequestRejectsSeedanceRemixOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newContext := func(modelName string) *gin.Context {
		request := httptest.NewRequest(http.MethodPost, "/v1/videos/task/remix", strings.NewReader(`{"model":"`+modelName+`","prompt":"change the scene"}`))
		request.Header.Set("Content-Type", "application/json")
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		return context
	}

	seedanceInfo := &relaycommon.RelayInfo{
		OriginModelName: relaycommon.SeedanceVideoModelStandard,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: "remix"},
	}
	seedanceErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(newContext(relaycommon.SeedanceVideoModelStandard), seedanceInfo)
	require.NotNil(t, seedanceErr)
	assert.Equal(t, http.StatusBadRequest, seedanceErr.StatusCode)

	legacyInfo := &relaycommon.RelayInfo{
		OriginModelName: "sora-2",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: "remix"},
	}
	assert.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(newContext("sora-2"), legacyInfo))
}

func TestEstimateBillingKeepsLegacySoraRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		req  relaycommon.TaskSubmitReq
		want map[string]float64
	}{
		{
			name: "sora 2 keeps seconds and size",
			req: relaycommon.TaskSubmitReq{
				Model:   "sora-2",
				Seconds: "8",
				Size:    "1280x720",
			},
			want: map[string]float64{
				"seconds": 8,
				"size":    1,
			},
		},
		{
			name: "sora 2 pro keeps large size ratio",
			req: relaycommon.TaskSubmitReq{
				Model:    "sora-2-pro",
				Duration: 8,
				Size:     "1792x1024",
			},
			want: map[string]float64{
				"seconds": 8,
				"size":    1.666667,
			},
		},
		{
			name: "similar videos prefix keeps legacy defaults",
			req: relaycommon.TaskSubmitReq{
				Model:   "videos-fast-preview",
				Seconds: "invalid",
			},
			want: map[string]float64{
				"seconds": 4,
				"size":    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratios := estimateBillingRatios(t, tt.req)
			assert.Equal(t, tt.want, ratios)
			assert.NotContains(t, ratios, "resolution")
		})
	}
}

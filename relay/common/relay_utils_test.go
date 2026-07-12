package common

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTaskValidationContext(t *testing.T, body string) (*gin.Context, *RelayInfo) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	return context, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
}

func TestSanitizeURLForLogMasksSensitiveQueryValues(t *testing.T) {
	rawURL := "https://example.test/v1beta/models/gemini:streamGenerateContent?alt=sse&key=sk-secret&access_token=ya29-secret&api-version=2024-02-01"

	got := SanitizeURLForLog(rawURL)

	assert.NotContains(t, got, "sk-secret")
	assert.NotContains(t, got, "ya29-secret")
	parsedURL, err := url.Parse(got)
	require.NoError(t, err)
	query := parsedURL.Query()
	assert.Equal(t, "***masked***", query.Get("key"))
	assert.Equal(t, "***masked***", query.Get("access_token"))
	assert.Equal(t, "sse", query.Get("alt"))
	assert.Equal(t, "2024-02-01", query.Get("api-version"))
}

func TestSanitizeURLForLogMasksAWSAndSecretLikeQueryKeys(t *testing.T) {
	rawURL := "https://example.test/path?X-Amz-Credential=credential&X-Amz-Signature=signature&session_token=session&client_secret=secret&model=gpt-test"

	got := SanitizeURLForLog(rawURL)

	assert.NotContains(t, got, "X-Amz-Credential=credential")
	assert.NotContains(t, got, "X-Amz-Signature=signature")
	assert.NotContains(t, got, "session_token=session")
	assert.NotContains(t, got, "client_secret=secret")
	parsedURL, err := url.Parse(got)
	require.NoError(t, err)
	query := parsedURL.Query()
	assert.Equal(t, "***masked***", query.Get("X-Amz-Credential"))
	assert.Equal(t, "***masked***", query.Get("X-Amz-Signature"))
	assert.Equal(t, "***masked***", query.Get("session_token"))
	assert.Equal(t, "***masked***", query.Get("client_secret"))
	assert.Equal(t, "gpt-test", query.Get("model"))
}

func TestSanitizeURLForLogKeepsURLWithoutSensitiveQuery(t *testing.T) {
	rawURL := "https://example.test/v1/chat/completions?api-version=2024-02-01&alt=sse"

	got := SanitizeURLForLog(rawURL)

	assert.Equal(t, rawURL, got)
}

func TestValidateMultipartDirectNormalizesImageField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.NewReader(`{"model":"wan2.7-i2v","prompt":"animate","image":" https://example.com/first.png "}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	info := &RelayInfo{
		TaskRelayInfo: &TaskRelayInfo{},
	}

	taskErr := ValidateMultipartDirect(context, info)

	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/first.png"}, storedReq.Images)
	require.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestValidateBasicTaskRequestKeepsLegacyMultipartSeconds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "vidu-q1"))
	require.NoError(t, writer.WriteField("prompt", "animate"))
	require.NoError(t, writer.WriteField("seconds", "9"))
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
	storage, err := appcommon.GetBodyStorage(context)
	require.NoError(t, err)
	context.Request.Body = io.NopCloser(storage)

	taskErr := ValidateBasicTaskRequest(context, info, constant.TaskActionGenerate)

	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	assert.Equal(t, 9, storedReq.Duration)
}

func TestValidateMultipartDirectSeedanceVideoRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, info := newTaskValidationContext(t, `{
		"model":"videos-standard",
		"prompt":"make a short film",
		"duration":"15",
		"ratio":"16:9",
		"resolution":" 4K ",
		"referenceImages":["https://example.com/image.png"],
		"referenceVideos":["https://example.com/video.mp4"],
		"referenceAudios":["https://example.com/audio.mp3"]
	}`)

	taskErr := ValidateMultipartDirect(context, info)

	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	assert.Equal(t, 15, storedReq.Duration)
	assert.Equal(t, "16:9", storedReq.Ratio)
	assert.Equal(t, "4k", storedReq.Resolution)
	assert.Equal(t, []string{"https://example.com/image.png"}, storedReq.ReferenceImages)
	assert.Equal(t, []string{"https://example.com/video.mp4"}, storedReq.ReferenceVideos)
	assert.Equal(t, []string{"https://example.com/audio.mp3"}, storedReq.ReferenceAudios)
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestValidateMultipartDirectCanonicalizesSeedanceVideoDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name         string
		body         string
		wantDuration int
	}{
		{
			name:         "missing duration uses four seconds",
			body:         `{"model":"videos-fast","prompt":"a cat","resolution":"480p"}`,
			wantDuration: 4,
		},
		{
			name:         "seconds is accepted as a compatibility fallback",
			body:         `{"model":"videos-mini","prompt":"a cat","seconds":"6","resolution":"720p"}`,
			wantDuration: 6,
		},
		{
			name:         "matching duration and seconds is accepted",
			body:         `{"model":"videos-standard","prompt":"a cat","duration":7,"seconds":"7","resolution":"1080p"}`,
			wantDuration: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, info := newTaskValidationContext(t, tt.body)

			taskErr := ValidateMultipartDirect(context, info)

			require.Nil(t, taskErr)
			storedReq, err := GetTaskRequest(context)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDuration, storedReq.Duration)
			assert.Empty(t, storedReq.Seconds)
		})
	}
}

func TestValidateMultipartDirectRejectsInvalidSeedanceVideoParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{
			name:     "fractional duration",
			body:     `{"model":"videos-fast","prompt":"a cat","duration":4.5,"resolution":"480p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "empty duration string",
			body:     `{"model":"videos-fast","prompt":"a cat","duration":"","resolution":"480p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "null duration",
			body:     `{"model":"videos-fast","prompt":"a cat","duration":null,"resolution":"480p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "overflowing duration string",
			body:     `{"model":"videos-fast","prompt":"a cat","duration":"999999999999999999999999","resolution":"480p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "unparsable seconds",
			body:     `{"model":"videos-fast","prompt":"a cat","seconds":"four","resolution":"480p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "duration and seconds conflict",
			body:     `{"model":"videos-fast","prompt":"a cat","duration":5,"seconds":"4","resolution":"480p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "duration below minimum",
			body:     `{"model":"videos-mini","prompt":"a cat","duration":3,"resolution":"720p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "duration above maximum",
			body:     `{"model":"videos-standard","prompt":"a cat","duration":16,"resolution":"1080p"}`,
			wantCode: "invalid_seconds",
		},
		{
			name:     "missing resolution",
			body:     `{"model":"videos-standard","prompt":"a cat","duration":4}`,
			wantCode: "invalid_resolution",
		},
		{
			name:     "fast unsupported resolution",
			body:     `{"model":"videos-fast","prompt":"a cat","duration":4,"resolution":"1080p"}`,
			wantCode: "invalid_resolution",
		},
		{
			name:     "standard unsupported resolution",
			body:     `{"model":"videos-standard","prompt":"a cat","duration":4,"resolution":"8k"}`,
			wantCode: "invalid_resolution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, info := newTaskValidationContext(t, tt.body)

			taskErr := ValidateMultipartDirect(context, info)

			require.NotNil(t, taskErr)
			assert.Equal(t, tt.wantCode, taskErr.Code)
		})
	}
}

func TestValidateMultipartDirectKeepsLegacyDurationCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		body       string
		wantAction string
	}{
		{
			name:       "sora fractional duration remains lenient",
			body:       `{"model":"sora-2","prompt":"a cat","duration":4.5}`,
			wantAction: constant.TaskActionTextGenerate,
		},
		{
			name:       "sora unparsable seconds remains lenient",
			body:       `{"model":"sora-2","prompt":"a cat","seconds":"four"}`,
			wantAction: constant.TaskActionTextGenerate,
		},
		{
			name:       "similar model name does not enter seedance validation",
			body:       `{"model":"videos-fast-preview","prompt":"a cat","duration":"invalid"}`,
			wantAction: constant.TaskActionTextGenerate,
		},
		{
			name:       "legacy missing duration remains accepted",
			body:       `{"model":"sora-2","prompt":"a cat"}`,
			wantAction: constant.TaskActionTextGenerate,
		},
		{
			name:       "legacy reference images do not change action",
			body:       `{"model":"sora-2","prompt":"a cat","referenceImages":["https://example.com/image.png"]}`,
			wantAction: constant.TaskActionTextGenerate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, info := newTaskValidationContext(t, tt.body)

			taskErr := ValidateMultipartDirect(context, info)

			require.Nil(t, taskErr)
			assert.Equal(t, tt.wantAction, info.Action)
		})
	}
}

// TestTaskDurationBounds guards the billing invariant that user-supplied
// video duration (a quota multiplier via OtherRatio "seconds") is bounded, so
// it can never overflow quota calculation into a negative charge.
func TestTaskDurationBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "huge duration is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","duration":9999999999}`,
			wantErr: true,
		},
		{
			name:    "huge seconds string is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","seconds":"9999999999"}`,
			wantErr: true,
		},
		{
			name:    "negative duration is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","duration":-8}`,
			wantErr: true,
		},
		{
			name: "normal duration is accepted",
			body: `{"model":"sora-2","prompt":"a cat","seconds":"8"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (multipart direct)", func(t *testing.T) {
			context, info := newTaskValidationContext(t, tt.body)
			taskErr := ValidateMultipartDirect(context, info)
			if tt.wantErr {
				require.NotNil(t, taskErr)
				require.Equal(t, "invalid_seconds", taskErr.Code)
			} else {
				require.Nil(t, taskErr)
			}
		})
		t.Run(tt.name+" (basic task request)", func(t *testing.T) {
			context, info := newTaskValidationContext(t, tt.body)
			taskErr := ValidateBasicTaskRequest(context, info, constant.TaskActionGenerate)
			if tt.wantErr {
				require.NotNil(t, taskErr)
				require.Equal(t, "invalid_seconds", taskErr.Code)
			} else {
				require.Nil(t, taskErr)
			}
		})
	}
}

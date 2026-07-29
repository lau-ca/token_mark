package xai

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func newXAIRequestContext(t *testing.T, requestJSON string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(requestJSON))
	c.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	return c, info
}

func validateXAIRequest(t *testing.T, requestJSON string) *dto.TaskError {
	t.Helper()
	c, info := newXAIRequestContext(t, requestJSON)
	return (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
}

func buildXAIRequestBody(t *testing.T, requestJSON string) map[string]any {
	t.Helper()
	c, info := newXAIRequestContext(t, requestJSON)
	adaptor := &TaskAdaptor{}
	if taskErr := adaptor.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("ValidateRequestAndSetAction() error = %v", taskErr)
	}
	body, err := adaptor.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("BuildRequestBody() error = %v", err)
	}
	payloadBytes, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	var payload map[string]any
	if err = json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, payloadBytes)
	}
	return payload
}

func TestBuildRequestBodyAcceptsOfficialImageInputs(t *testing.T) {
	tests := []struct {
		name        string
		requestJSON string
		wantURL     string
		wantFileID  string
	}{
		{"public URL object", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":{"url":"https://example.com/frame.png"}}`, "https://example.com/frame.png", ""},
		{"base64 object", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":{"url":"data:image/png;base64,cG5n"}}`, "data:image/png;base64,cG5n", ""},
		{"file ID object", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":{"file_id":"file_123"}}`, "", "file_123"},
		{"legacy string", `{"model":"grok-imagine-video-1.5-preview","prompt":"animate","duration":7,"image":"https://example.com/legacy.png"}`, "https://example.com/legacy.png", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := buildXAIRequestBody(t, test.requestJSON)
			image, ok := payload["image"].(map[string]any)
			if !ok {
				t.Fatalf("image = %#v, want object", payload["image"])
			}
			if test.wantURL != "" && image["url"] != test.wantURL {
				t.Fatalf("image.url = %#v, want %q", image["url"], test.wantURL)
			}
			if test.wantFileID != "" && image["file_id"] != test.wantFileID {
				t.Fatalf("image.file_id = %#v, want %q", image["file_id"], test.wantFileID)
			}
		})
	}
}

func TestValidateRequestRejectsInvalidOfficialImageInputs(t *testing.T) {
	tests := []string{
		`{"model":"grok-imagine-video","prompt":"animate","image":{}}`,
		`{"model":"grok-imagine-video","prompt":"animate","image":{"url":"https://example.com/a.png","file_id":"file_123"}}`,
		`{"model":"grok-imagine-video","prompt":"animate","image":{"url":"https://example.com/a.png"},"images":["https://example.com/b.png"]}`,
	}

	for _, requestJSON := range tests {
		t.Run(requestJSON, func(t *testing.T) {
			taskErr := validateXAIRequest(t, requestJSON)
			if taskErr == nil || taskErr.StatusCode != http.StatusBadRequest {
				t.Fatalf("taskErr = %#v, want HTTP 400", taskErr)
			}
		})
	}
}

func TestBuildRequestBodyKeepsMultipartImageUpload(t *testing.T) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	if err := writer.WriteField("model", "grok-imagine-video-1.5-preview"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("prompt", "animate"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("seconds", "4"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("image", "frame.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte("png-data")); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &requestBody)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video-1.5-preview"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	adaptor := &TaskAdaptor{}
	if taskErr := adaptor.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("ValidateRequestAndSetAction() error = %v", taskErr)
	}
	body, err := adaptor.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("BuildRequestBody() error = %v", err)
	}
	payloadBytes, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, payloadBytes)
	}
	image, ok := payload["image"].(map[string]any)
	imageURL, _ := image["url"].(string)
	if !ok || !strings.HasPrefix(imageURL, "data:") {
		t.Fatalf("image = %#v, want data URL", payload["image"])
	}
}

func TestBuildRequestBodyForwardsVideoOptions(t *testing.T) {
	tests := []struct {
		name            string
		requestJSON     string
		wantDuration    float64
		wantAspectRatio string
		wantResolution  string
	}{
		{
			name:            "standard aspect ratio takes precedence",
			requestJSON:     `{"model":"grok-imagine-video","prompt":"animate","seconds":"7","aspect_ratio":"9:16","ratio":"1:1","size":"1280x720","resolution":"720p"}`,
			wantDuration:    7,
			wantAspectRatio: "9:16",
			wantResolution:  "720p",
		},
		{
			name:            "ratio compatibility alias",
			requestJSON:     `{"model":"grok-imagine-video","prompt":"animate","duration":6,"ratio":"3:2"}`,
			wantDuration:    6,
			wantAspectRatio: "3:2",
		},
		{
			name:            "portrait size mapping",
			requestJSON:     `{"model":"grok-imagine-video","prompt":"animate","duration":5,"size":"720x1280"}`,
			wantDuration:    5,
			wantAspectRatio: "9:16",
		},
		{
			name:            "landscape size mapping",
			requestJSON:     `{"model":"grok-imagine-video","prompt":"animate","duration":4,"size":"1792x1024"}`,
			wantDuration:    4,
			wantAspectRatio: "16:9",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := buildXAIRequestBody(t, test.requestJSON)
			if payload["duration"] != test.wantDuration {
				t.Fatalf("duration = %#v, want %v", payload["duration"], test.wantDuration)
			}
			if payload["aspect_ratio"] != test.wantAspectRatio {
				t.Fatalf("aspect_ratio = %#v, want %q", payload["aspect_ratio"], test.wantAspectRatio)
			}
			if test.wantResolution != "" && payload["resolution"] != test.wantResolution {
				t.Fatalf("resolution = %#v, want %q", payload["resolution"], test.wantResolution)
			}
		})
	}
}

func TestBuildRequestBodyOmitsUnsetVideoOptions(t *testing.T) {
	payload := buildXAIRequestBody(t, `{"model":"grok-imagine-video","prompt":"animate","duration":4}`)
	if _, exists := payload["aspect_ratio"]; exists {
		t.Fatalf("aspect_ratio should be omitted: %#v", payload)
	}
	if _, exists := payload["resolution"]; exists {
		t.Fatalf("resolution should be omitted: %#v", payload)
	}
}

func TestParseTaskResultDone(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{
		"status": "done",
		"video": {
			"url": "https://example.com/video.mp4",
			"duration": 15
		},
		"progress": 100
	}`))
	if err != nil {
		t.Fatalf("ParseTaskResult returned error: %v", err)
	}

	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("status = %s, want %s", result.Status, model.TaskStatusSuccess)
	}
	if result.Url != "https://example.com/video.mp4" {
		t.Fatalf("url = %q, want video url", result.Url)
	}
	if result.Progress != "100%" {
		t.Fatalf("progress = %q, want 100%%", result.Progress)
	}
}

func TestParseTaskResultPending(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{"status":"pending","progress":40}`))
	if err != nil {
		t.Fatalf("ParseTaskResult returned error: %v", err)
	}

	if result.Status != model.TaskStatusInProgress {
		t.Fatalf("status = %s, want %s", result.Status, model.TaskStatusInProgress)
	}
	if result.Progress != "40%" {
		t.Fatalf("progress = %q, want 40%%", result.Progress)
	}
}

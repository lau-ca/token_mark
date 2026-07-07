package xai

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

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

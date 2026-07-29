package xai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const videoImageRefContextKey = "xai_video_image_ref"

type imageRef struct {
	URL    string `json:"url,omitempty"`
	FileID string `json:"file_id,omitempty"`
}

func setVideoImageRef(c *gin.Context, ref *imageRef) {
	if c != nil && ref != nil {
		c.Set(videoImageRefContextKey, *ref)
	}
}

func getVideoImageRef(c *gin.Context) (*imageRef, bool) {
	if c == nil {
		return nil, false
	}
	value, exists := c.Get(videoImageRefContextKey)
	if !exists {
		return nil, false
	}
	ref, ok := value.(imageRef)
	if !ok {
		return nil, false
	}
	return &ref, true
}

func parseVideoTaskRequest(c *gin.Context) (relaycommon.TaskSubmitReq, *imageRef, error) {
	var fields map[string]json.RawMessage
	if err := common.UnmarshalBodyReusable(c, &fields); err != nil {
		return relaycommon.TaskSubmitReq{}, nil, err
	}

	imageJSON, hasImage := fields["image"]
	delete(fields, "image")
	normalizedJSON, err := common.Marshal(fields)
	if err != nil {
		return relaycommon.TaskSubmitReq{}, nil, err
	}
	var req relaycommon.TaskSubmitReq
	if err = common.Unmarshal(normalizedJSON, &req); err != nil {
		return relaycommon.TaskSubmitReq{}, nil, err
	}
	if !hasImage || strings.TrimSpace(string(imageJSON)) == "null" {
		return req, nil, nil
	}

	ref, err := parseVideoImageRef(imageJSON)
	if err != nil {
		return relaycommon.TaskSubmitReq{}, nil, err
	}
	if req.InputReference != "" || len(req.Images) > 0 {
		return relaycommon.TaskSubmitReq{}, nil, fmt.Errorf("image cannot be combined with input_reference or images")
	}
	if ref.URL != "" {
		req.Image = ref.URL
	}
	return req, ref, nil
}

func parseVideoImageRef(raw json.RawMessage) (*imageRef, error) {
	var legacyURL string
	if err := common.Unmarshal(raw, &legacyURL); err == nil {
		legacyURL = strings.TrimSpace(legacyURL)
		if legacyURL == "" {
			return nil, fmt.Errorf("image string must not be empty")
		}
		return &imageRef{URL: legacyURL}, nil
	}

	var ref imageRef
	if err := common.Unmarshal(raw, &ref); err != nil {
		return nil, fmt.Errorf("image must be a string or an object: %w", err)
	}
	ref.URL = strings.TrimSpace(ref.URL)
	ref.FileID = strings.TrimSpace(ref.FileID)
	if (ref.URL == "") == (ref.FileID == "") {
		return nil, fmt.Errorf("image must provide exactly one of url or file_id")
	}
	return &ref, nil
}

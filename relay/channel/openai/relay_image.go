package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// OpenaiImageHandler handles non-streaming OpenAI image responses
// (generations/edits), returning the parsed usage for billing.
func OpenaiImageHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var usageResp dto.SimpleResponse
	err = common.Unmarshal(responseBody, &usageResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responseBody = stripChannelImageURLs(responseBody, info)

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	normalizeOpenAIUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)
	return &usageResp.Usage, nil
}

// normalizeOpenAIUsage maps the OpenAI Images usage shape (input_tokens /
// output_tokens / input_tokens_details) onto the canonical prompt/completion
// fields. It is used only on the OpenAI image relay paths (generations/edits,
// streaming and non-streaming): the image API never returns prompt_tokens /
// completion_tokens, so the overwrite (=) semantics here are equivalent to the
// previous additive (+=) behavior while avoiding any future double-counting if
// both field sets are ever populated. Do not reuse this on chat/embedding paths
// without revisiting the overwrite semantics.
func normalizeOpenAIUsage(usage *dto.Usage) {
	if usage == nil {
		return
	}
	if usage.InputTokens != 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.OutputTokens != 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.InputTokensDetails != nil {
		usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
		usage.PromptTokensDetails.CachedCreationTokens = usage.InputTokensDetails.CachedCreationTokens
		usage.PromptTokensDetails.ImageTokens = usage.InputTokensDetails.ImageTokens
		usage.PromptTokensDetails.TextTokens = usage.InputTokensDetails.TextTokens
		usage.PromptTokensDetails.AudioTokens = usage.InputTokensDetails.AudioTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
}

func OpenaiImageStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid image stream response")
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return OpenaiImageHandler(c, info, resp)
	}
	if !strings.Contains(contentType, "text/event-stream") {
		return OpenaiImageJSONAsStreamHandler(c, info, resp)
	}
	// Reuse the shared streaming engine (helper.StreamScannerHandler) so the
	// image streaming path gets the same ping keepalive, streaming-timeout
	// watchdog, client-disconnect detection, panic recovery and goroutine
	// cleanup as every other relay stream. The scanner delivers only the
	// "data:" payload, so the SSE "event:" line is rebuilt from the JSON "type"
	// field (real OpenAI image events keep event == type).
	usage := &dto.Usage{}
	var lastStreamData []byte

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		raw := common.StringToByteSlice(data)
		lastStreamData = raw
		if isOpenAIImageStreamErrorEvent(raw) {
			// Record the error as a soft error; the scanner drives the final
			// EndReason. HasErrors() flags the failure for logging/handling.
			sr.Error(fmt.Errorf("%s", extractOpenAIImageStreamErrorMessage(raw)))
		}
		var usageResp dto.SimpleResponse
		if err := common.Unmarshal(raw, &usageResp); err == nil {
			normalizeOpenAIUsage(&usageResp.Usage)
			if service.ValidUsage(&usageResp.Usage) {
				usage = &usageResp.Usage
			}
		}
		writeOpenaiImageStreamChunk(c, info, raw)
	})

	// StreamScannerHandler consumes the upstream [DONE]; re-emit it so the
	// client still receives a terminal data: [DONE].
	if info != nil && info.StreamStatus != nil && info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone {
		helper.Done(c)
	}

	applyUsagePostProcessing(info, usage, lastStreamData)
	return usage, nil
}

// writeOpenaiImageStreamChunk rebuilds the SSE frame for an image stream chunk:
// it emits an "event:" line derived from the JSON "type" field (when present)
// followed by the verbatim "data:" payload, mirroring helper.ResponseChunkData.
func writeOpenaiImageStreamChunk(c *gin.Context, info *relaycommon.RelayInfo, data []byte) {
	data = stripChannelImageURLs(data, info)
	var payload struct {
		Type string `json:"type"`
	}
	_ = common.Unmarshal(data, &payload)
	if eventName := strings.TrimSpace(payload.Type); eventName != "" {
		_ = helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventName}, string(data))
		return
	}
	_ = helper.StringData(c, string(data))
}

// isOpenAIImageStreamErrorEvent detects upstream error chunks by JSON content
// only ("type" of error/upstream_error, or a non-empty "error" field). The SSE
// "event:" line is not available here: StreamScannerHandler delivers only the
// "data:" payload. A payload carrying just a "message" key is deliberately NOT
// treated as an error to avoid false positives.
func isOpenAIImageStreamErrorEvent(data []byte) bool {
	if !json.Valid(data) {
		return false
	}
	var payload struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return false
	}
	payloadType := strings.ToLower(strings.TrimSpace(payload.Type))
	return payloadType == "error" || payloadType == "upstream_error" || len(payload.Error) > 0
}

func extractOpenAIImageStreamErrorMessage(data []byte) string {
	if len(data) == 0 || !json.Valid(data) {
		return "upstream image stream returned error event"
	}
	var payload struct {
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return "upstream image stream returned error event"
	}
	if msg := strings.TrimSpace(payload.Message); msg != "" {
		return msg
	}
	if len(payload.Error) > 0 {
		var nested struct {
			Message string `json:"message"`
		}
		if err := common.Unmarshal(payload.Error, &nested); err == nil {
			if msg := strings.TrimSpace(nested.Message); msg != "" {
				return msg
			}
		}
		if msg := strings.TrimSpace(common.JsonRawMessageToString(payload.Error)); msg != "" {
			return msg
		}
	}
	return "upstream image stream returned error event"
}

func OpenaiImageJSONAsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	responseBody = stripChannelImageURLs(responseBody, info)
	var imageResp dto.ImageResponse
	if err := common.Unmarshal(responseBody, &imageResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var usageResp dto.SimpleResponse
	_ = common.Unmarshal(responseBody, &usageResp)
	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}
	normalizeOpenAIUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)

	helper.SetEventStreamHeaders(c)
	c.Status(http.StatusOK)

	created := imageResp.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	if info != nil {
		info.SetFirstResponseTime()
	}
	for _, image := range imageResp.Data {
		payload := map[string]any{
			"type":       "image_generation.completed",
			"created_at": created,
		}
		if image.Url != "" {
			payload["url"] = image.Url
		}
		if image.B64Json != "" {
			payload["b64_json"] = image.B64Json
		}
		if image.RevisedPrompt != "" {
			payload["revised_prompt"] = image.RevisedPrompt
		}
		if service.ValidUsage(&usageResp.Usage) {
			payload["usage"] = usageResp.Usage
		}
		if err := writeOpenaiImageStreamPayload(c, "image_generation.completed", payload); err != nil {
			if info != nil && info.StreamStatus != nil {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
			}
			return &usageResp.Usage, nil
		}
	}
	if err := writeOpenaiImageStreamDone(c); err != nil {
		if info != nil && info.StreamStatus != nil {
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
		}
		return &usageResp.Usage, nil
	}
	if info != nil {
		info.ReceivedResponseCount += len(imageResp.Data)
		if info.StreamStatus == nil {
			info.StreamStatus = relaycommon.NewStreamStatus()
		}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	}
	return &usageResp.Usage, nil
}

func stripChannelImageURLs(data []byte, info *relaycommon.RelayInfo) []byte {
	if !shouldStripChannelImageURL(info) || len(data) == 0 {
		return data
	}
	return stripJSONField(data, "url")
}

func shouldStripChannelImageURL(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	return info.ChannelOtherSettings.ForceImageB64JSONNoURL
}

func stripJSONField(data []byte, field string) []byte {
	quotedField := []byte(`"` + field + `"`)
	var out []byte
	lastWrite := 0

	for i := 0; i < len(data); {
		if data[i] != '"' {
			i++
			continue
		}

		if !matchesJSONField(data, i, quotedField) {
			next := skipJSONString(data, i)
			if next <= i {
				i++
			} else {
				i = next
			}
			continue
		}

		fieldEnd := i + len(quotedField)
		colon := skipJSONSpaces(data, fieldEnd)
		valueStart := skipJSONSpaces(data, colon+1)
		valueEnd := skipJSONValue(data, valueStart)
		if valueEnd <= valueStart {
			i = fieldEnd
			continue
		}

		removeStart, removeEnd := i, valueEnd
		next := skipJSONSpaces(data, valueEnd)
		if next < len(data) && data[next] == ',' {
			removeEnd = next + 1
		} else if prev := prevJSONNonSpace(data, i-1); prev >= 0 && data[prev] == ',' {
			removeStart = prev
		}

		if out == nil {
			out = make([]byte, 0, len(data)-(removeEnd-removeStart))
		}
		out = append(out, data[lastWrite:removeStart]...)
		lastWrite = removeEnd
		i = removeEnd
	}

	if out == nil {
		return data
	}
	out = append(out, data[lastWrite:]...)
	return out
}

func matchesJSONField(data []byte, pos int, quotedField []byte) bool {
	if pos+len(quotedField) > len(data) || !bytes.Equal(data[pos:pos+len(quotedField)], quotedField) {
		return false
	}
	prev := prevJSONNonSpace(data, pos-1)
	if prev < 0 || (data[prev] != '{' && data[prev] != ',') {
		return false
	}
	next := skipJSONSpaces(data, pos+len(quotedField))
	return next < len(data) && data[next] == ':'
}

func prevJSONNonSpace(data []byte, pos int) int {
	for pos >= 0 && isJSONSpace(data[pos]) {
		pos--
	}
	return pos
}

func skipJSONSpaces(data []byte, pos int) int {
	for pos < len(data) && isJSONSpace(data[pos]) {
		pos++
	}
	return pos
}

func isJSONSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func skipJSONString(data []byte, pos int) int {
	if pos >= len(data) || data[pos] != '"' {
		return pos
	}
	for i := pos + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return len(data)
}

func skipJSONValue(data []byte, pos int) int {
	pos = skipJSONSpaces(data, pos)
	if pos >= len(data) {
		return pos
	}
	if data[pos] == '"' {
		return skipJSONString(data, pos)
	}
	if data[pos] != '{' && data[pos] != '[' {
		for pos < len(data) && data[pos] != ',' && data[pos] != '}' && data[pos] != ']' {
			pos++
		}
		return pos
	}

	stack := []byte{data[pos]}
	for i := pos + 1; i < len(data); i++ {
		switch data[i] {
		case '"':
			i = skipJSONString(data, i) - 1
		case '{', '[':
			stack = append(stack, data[i])
		case '}', ']':
			if len(stack) == 0 {
				return i
			}
			open := stack[len(stack)-1]
			if (open == '{' && data[i] != '}') || (open == '[' && data[i] != ']') {
				return i
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1
			}
		}
	}
	return len(data)
}

func writeOpenaiImageStreamPayload(c *gin.Context, eventName string, payload any) error {
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	if eventName != "" {
		return helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventName}, string(data))
	}
	return helper.StringData(c, string(data))
}

func writeOpenaiImageStreamDone(c *gin.Context) error {
	return helper.StringData(c, "[DONE]")
}

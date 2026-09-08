package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func updateOpenAIImageCount(info *relaycommon.RelayInfo, count int64) {
	if info == nil || !info.PriceData.UsePrice || count <= 0 || count > int64(dto.MaxImageN) {
		return
	}
	info.PriceData.AddOtherRatio("n", float64(count))
}

// OpenaiImageHandler handles non-streaming OpenAI image responses
// (generations/edits), returning the parsed usage for billing.
func OpenaiImageHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	normalizeResponse := shouldNormalizeOpenAIImageResponse(info) &&
		resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	usageResp, oaiError, err := decodeOpenAIImageResponse(responseBody, normalizeResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}
	updateOpenAIImageCount(info, gjson.GetBytes(responseBody, "data.#").Int())
	responseBody = stripChannelImageURLs(responseBody, info)
	responseBody, err = relaycommon.ApplyResponseParamOverrideWithRelayInfo(responseBody, info)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
	}
	if normalizeResponse {
		responseBody, err = normalizeOpenAIImageResponse(responseBody, info)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}
	if validationErr := validateChannelImageResponseURLs(responseBody, info); validationErr != nil {
		return nil, validationErr
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	normalizeOpenAIUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)
	return &usageResp.Usage, nil
}

type normalizedImageTokenDetails struct {
	ImageTokens int `json:"image_tokens"`
	TextTokens  int `json:"text_tokens"`
}

type normalizedImageUsage struct {
	InputTokens         int                         `json:"input_tokens"`
	InputTokensDetails  normalizedImageTokenDetails `json:"input_tokens_details"`
	OutputTokens        int                         `json:"output_tokens"`
	TotalTokens         int                         `json:"total_tokens"`
	OutputTokensDetails normalizedImageTokenDetails `json:"output_tokens_details"`
}

func decodeOpenAIImageResponse(body []byte, normalizeResponse bool) (dto.SimpleResponse, *types.OpenAIError, error) {
	var response dto.SimpleResponse
	if !normalizeResponse {
		if err := common.Unmarshal(body, &response); err != nil {
			return response, nil, err
		}
		return response, response.GetOpenAIError(), nil
	}

	var envelope struct {
		Error any `json:"error"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return response, nil, err
	}
	response.Usage = normalizedImageUsageToDTO(buildNormalizedImageUsage(body))
	return response, dto.GetOpenAIError(envelope.Error), nil
}

func normalizedImageUsageToDTO(usage normalizedImageUsage) dto.Usage {
	return dto.Usage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		InputTokensDetails: &dto.InputTokenDetails{
			ImageTokens: usage.InputTokensDetails.ImageTokens,
			TextTokens:  usage.InputTokensDetails.TextTokens,
		},
		CompletionTokenDetails: dto.OutputTokenDetails{
			ImageTokens: usage.OutputTokensDetails.ImageTokens,
			TextTokens:  usage.OutputTokensDetails.TextTokens,
		},
	}
}

func normalizeOpenAIImageResponse(body []byte, info *relaycommon.RelayInfo) ([]byte, error) {
	if !shouldNormalizeOpenAIImageResponse(info) {
		return body, nil
	}
	if !gjson.ParseBytes(body).IsObject() {
		return nil, fmt.Errorf("invalid OpenAI image response: expected JSON object")
	}

	request := &dto.ImageRequest{}
	if info != nil {
		if imageRequest, ok := info.Request.(*dto.ImageRequest); ok && imageRequest != nil {
			request = imageRequest
		}
	}

	requestOutputFormat := ""
	if len(request.OutputFormat) > 0 {
		_ = common.Unmarshal(request.OutputFormat, &requestOutputFormat)
	}
	outputFormat := imageResponseString(body, "output_format", requestOutputFormat, "png")
	size := imageResponseString(body, "size", request.Size, "auto")
	created, ok := imageResponseInt64(body, "created")
	if !ok {
		created = time.Now().Unix()
	}

	usageJSON, err := common.Marshal(buildNormalizedImageUsage(body))
	if err != nil {
		return nil, err
	}
	body, err = sjson.SetBytes(body, "created", created)
	if err != nil {
		return nil, err
	}
	body, err = sjson.SetBytes(body, "output_format", outputFormat)
	if err != nil {
		return nil, err
	}
	if !gjson.GetBytes(body, "quality").Exists() {
		quality := request.Quality
		if strings.TrimSpace(quality) == "" {
			quality = "medium"
		}
		body, err = sjson.SetBytes(body, "quality", quality)
		if err != nil {
			return nil, err
		}
	}
	body, err = sjson.SetBytes(body, "size", size)
	if err != nil {
		return nil, err
	}
	return sjson.SetRawBytes(body, "usage", usageJSON)
}

func shouldNormalizeOpenAIImageResponse(info *relaycommon.RelayInfo) bool {
	return info != nil && info.ChannelMeta != nil && info.ChannelOtherSettings.NormalizeOpenAIImageResponse
}

func imageResponseString(body []byte, path string, requestValue string, defaultValue string) string {
	upstreamValue := gjson.GetBytes(body, path)
	if upstreamValue.Type == gjson.String && strings.TrimSpace(upstreamValue.String()) != "" {
		return upstreamValue.String()
	}
	if strings.TrimSpace(requestValue) != "" {
		return requestValue
	}
	return defaultValue
}

func imageResponseInt64(body []byte, path string) (int64, bool) {
	value := gjson.GetBytes(body, path)
	if value.Type != gjson.Number {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value.Raw, 10, 64)
	if err != nil || parsed < 0 {
		return 0, false
	}
	return parsed, true
}

func buildNormalizedImageUsage(body []byte) normalizedImageUsage {
	return normalizedImageUsage{
		InputTokens: imageUsageInt(body, "usage.input_tokens", "usage.prompt_tokens"),
		InputTokensDetails: normalizedImageTokenDetails{
			ImageTokens: imageUsageInt(body, "usage.input_tokens_details.image_tokens", "usage.prompt_tokens_details.image_tokens"),
			TextTokens:  imageUsageInt(body, "usage.input_tokens_details.text_tokens", "usage.prompt_tokens_details.text_tokens"),
		},
		OutputTokens: imageUsageInt(body, "usage.output_tokens", "usage.completion_tokens"),
		TotalTokens:  imageUsageInt(body, "usage.total_tokens", ""),
		OutputTokensDetails: normalizedImageTokenDetails{
			ImageTokens: imageUsageInt(body, "usage.output_tokens_details.image_tokens", "usage.completion_tokens_details.image_tokens"),
			TextTokens:  imageUsageInt(body, "usage.output_tokens_details.text_tokens", "usage.completion_tokens_details.text_tokens"),
		},
	}
}

func imageUsageInt(body []byte, primaryPath string, fallbackPath string) int {
	for _, path := range []string{primaryPath, fallbackPath} {
		if path == "" {
			continue
		}
		value := gjson.GetBytes(body, path)
		if value.Type != gjson.Number {
			continue
		}
		parsed, err := strconv.ParseInt(value.Raw, 10, 0)
		if err == nil && parsed >= 0 {
			return int(parsed)
		}
	}
	return 0
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
		usage.PromptTokensDetails.CacheWriteTokens = usage.InputTokensDetails.CacheWriteTokens
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
		return openaiImageJSONAsStreamHandler(c, info, resp)
	}
	if shouldValidateChannelImageResponseURL(info) {
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
		}
		if validationErr := validateChannelImageResponseURLs(responseBody, info); validationErr != nil {
			return nil, validationErr
		}
		resp.Body = io.NopCloser(bytes.NewReader(responseBody))
	}
	// Reuse the shared streaming engine (helper.StreamScannerHandler) so the
	// image streaming path gets the same ping keepalive, streaming-timeout
	// watchdog, client-disconnect detection, panic recovery and goroutine
	// cleanup as every other relay stream. The scanner delivers only the
	// "data:" payload, so the SSE "event:" line is rebuilt from the JSON "type"
	// field (real OpenAI image events keep event == type).
	usage := &dto.Usage{}
	var lastStreamData []byte
	var completedImages int64

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		raw := common.StringToByteSlice(data)
		lastStreamData = raw
		if isOpenAIImageStreamErrorEvent(raw) {
			// Record the error as a soft error; the scanner drives the final
			// EndReason. HasErrors() flags the failure for logging/handling.
			sr.Error(fmt.Errorf("%s", extractOpenAIImageStreamErrorMessage(raw)))
		}
		var chunk struct {
			Type  string    `json:"type"`
			Usage dto.Usage `json:"usage"`
		}
		if err := common.Unmarshal(raw, &chunk); err == nil {
			normalizeOpenAIUsage(&chunk.Usage)
			if service.ValidUsage(&chunk.Usage) {
				usage = &chunk.Usage
			}
			if chunk.Type == "image_generation.completed" || chunk.Type == "image_edit.completed" {
				completedImages++
			}
		}
		if err := writeOpenaiImageStreamChunk(c, info, raw); err != nil {
			sr.Stop(err)
		}
	})

	// StreamScannerHandler consumes the upstream [DONE]; re-emit it so the
	// client still receives a terminal data: [DONE].
	if info.StreamStatus != nil && info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone {
		helper.Done(c)
	}

	applyUsagePostProcessing(info, usage, lastStreamData)
	// Only trust completedImages when upstream finished the stream (done/eof).
	// On client-side aborts (client_gone, or handler_stop from a failed client
	// write) the counter undercounts what upstream actually generated and
	// charged, so keep the requested n — otherwise a client could pay for one
	// image by disconnecting right after the first completed event. The abort
	// guard only blocks lowering the charge: if completed events already
	// exceed the recorded n, bill the higher actual count regardless.
	if info.StreamStatus != nil {
		upstreamFinished := info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone ||
			info.StreamStatus.EndReason == relaycommon.StreamEndReasonEOF
		requestedN := 1.0
		if n, ok := info.PriceData.OtherRatios()["n"]; ok {
			requestedN = n
		}
		if upstreamFinished || float64(completedImages) > requestedN {
			updateOpenAIImageCount(info, completedImages)
		}
	}
	return usage, nil
}

// writeOpenaiImageStreamChunk rebuilds the SSE frame for an image stream chunk:
// it emits an "event:" line derived from the JSON "type" field (when present)
// followed by the verbatim "data:" payload, mirroring helper.ResponseChunkData.
func writeOpenaiImageStreamChunk(c *gin.Context, info *relaycommon.RelayInfo, data []byte) error {
	data = stripChannelImageURLs(data, info)
	var payload struct {
		Type string `json:"type"`
	}
	_ = common.Unmarshal(data, &payload)
	if eventName := strings.TrimSpace(payload.Type); eventName != "" {
		return helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventName}, string(data))
	}
	return helper.StringData(c, string(data))
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

func openaiImageJSONAsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if validationErr := validateChannelImageResponseURLs(responseBody, info); validationErr != nil {
		return nil, validationErr
	}

	responseBody = stripChannelImageURLs(responseBody, info)
	// Only decode usage/error. Do not Unmarshal data[] into dto.ImageResponse —
	// b64_json values are large and would be copied into Go strings then
	// re-marshaled for each SSE event.
	var usageResp dto.SimpleResponse
	if err := common.Unmarshal(responseBody, &usageResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}
	normalizeOpenAIUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)

	imageCount := gjson.GetBytes(responseBody, "data.#").Int()
	updateOpenAIImageCount(info, imageCount)

	helper.SetEventStreamHeaders(c)
	c.Status(http.StatusOK)

	created := gjson.GetBytes(responseBody, "created").Int()
	if created == 0 {
		created = time.Now().Unix()
	}
	if info != nil {
		info.SetFirstResponseTime()
	}

	validUsage := service.ValidUsage(&usageResp.Usage)
	var usageJSON []byte
	if validUsage {
		usageJSON, err = common.Marshal(usageResp.Usage)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}

	for i := range imageCount {
		image := gjson.GetBytes(responseBody, "data."+strconv.FormatInt(i, 10))
		payload := []byte(`{"type":"image_generation.completed"}`)
		payload, err = sjson.SetBytes(payload, "created_at", created)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if validUsage {
			payload, err = sjson.SetRawBytes(payload, "usage", usageJSON)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		// b64_json goes last: every sjson.Set* reallocates the whole payload,
		// so inserting the large blob after all small fields avoids re-copying
		// multi-MB buffers.
		for _, field := range []string{"url", "revised_prompt", "b64_json"} {
			value := image.Get(field)
			if value.Type != gjson.String || value.Raw == `""` {
				continue
			}
			raw := []byte(value.Raw)
			if value.Index > 0 {
				raw = responseBody[value.Index : value.Index+len(value.Raw)]
			}
			payload, err = sjson.SetRawBytes(payload, field, raw)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if writeErr := helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "image_generation.completed"}, string(payload)); writeErr != nil {
			if info != nil && info.StreamStatus != nil {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, writeErr)
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
		info.ReceivedResponseCount += int(imageCount)
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
	return stripJSONFields(data, "url", "_provider_image_url")
}

func shouldStripChannelImageURL(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	return info.ChannelOtherSettings.ForceImageB64JSONNoURL
}

func shouldValidateChannelImageResponseURL(info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.ChannelMeta != nil &&
		!info.ChannelOtherSettings.ForceImageB64JSONNoURL &&
		strings.TrimSpace(info.ChannelOtherSettings.ImageResponseURLPrefix) != ""
}

func validateChannelImageResponseURLs(data []byte, info *relaycommon.RelayInfo) *types.NewAPIError {
	if !shouldValidateChannelImageResponseURL(info) || len(data) == 0 {
		return nil
	}

	prefix := strings.TrimSpace(info.ChannelOtherSettings.ImageResponseURLPrefix)
	for i := 0; i < len(data); {
		if data[i] != '"' {
			i++
			continue
		}

		matchedFieldLength := 0
		for _, field := range [][]byte{[]byte(`"url"`), []byte(`"_provider_image_url"`)} {
			if matchesJSONField(data, i, field) {
				matchedFieldLength = len(field)
				break
			}
		}
		if matchedFieldLength == 0 {
			next := skipJSONString(data, i)
			if next <= i {
				i++
			} else {
				i = next
			}
			continue
		}

		colon := skipJSONSpaces(data, i+matchedFieldLength)
		valueStart := skipJSONSpaces(data, colon+1)
		valueEnd := skipJSONValue(data, valueStart)
		if valueEnd <= valueStart || data[valueStart] != '"' {
			return invalidChannelImageResponseURLError()
		}
		urlValue := ""
		if err := common.Unmarshal(data[valueStart:valueEnd], &urlValue); err != nil || !strings.HasPrefix(urlValue, prefix) {
			return invalidChannelImageResponseURLError()
		}
		i = valueEnd
	}
	return nil
}

func invalidChannelImageResponseURLError() *types.NewAPIError {
	return types.NewOpenAIError(errors.New("openai error."), types.ErrorCodeBadResponse, http.StatusBadGateway)
}

func stripJSONFields(data []byte, fields ...string) []byte {
	quotedFields := make([][]byte, 0, len(fields))
	for _, field := range fields {
		if field != "" {
			quotedFields = append(quotedFields, []byte(`"`+field+`"`))
		}
	}
	if len(quotedFields) == 0 {
		return data
	}

	var out []byte
	lastWrite := 0

	for i := 0; i < len(data); {
		if data[i] != '"' {
			i++
			continue
		}

		matchedFieldLength := 0
		for _, quotedField := range quotedFields {
			if matchesJSONField(data, i, quotedField) {
				matchedFieldLength = len(quotedField)
				break
			}
		}
		if matchedFieldLength == 0 {
			next := skipJSONString(data, i)
			if next <= i {
				i++
			} else {
				i = next
			}
			continue
		}

		fieldEnd := i + matchedFieldLength
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

func writeOpenaiImageStreamDone(c *gin.Context) error {
	return helper.StringData(c, "[DONE]")
}

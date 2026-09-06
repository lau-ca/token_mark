package dto

import (
	"fmt"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

const (
	PlaygroundParameterString  = "string"
	PlaygroundParameterNumber  = "number"
	PlaygroundParameterBoolean = "boolean"
	PlaygroundParameterEnum    = "enum"
)

var playgroundCapabilities = map[string]struct{}{
	"chat":                 {},
	"image.generate":       {},
	"image.edit":           {},
	"video.text_to_video":  {},
	"video.image_to_video": {},
}

type PlaygroundParameter struct {
	Key         string   `json:"key"`
	Label       string   `json:"label,omitempty"`
	Type        string   `json:"type"`
	RequestPath string   `json:"request_path,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []any    `json:"options,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
}

func ValidatePlaygroundParameterValues(config *ModelPlaygroundConfig, values map[string]any) error {
	if config == nil {
		return nil
	}
	for _, parameter := range config.Parameters {
		value, exists := values[parameter.Key]
		if !exists && parameter.RequestPath != "" {
			value, exists = GetPlaygroundParameterValue(values, parameter.RequestPath)
		}
		if !exists || value == nil || value == "" {
			if parameter.Required {
				return fmt.Errorf("parameter %s is required", parameter.Key)
			}
			continue
		}

		if err := validatePlaygroundParameterValue(parameter, value); err != nil {
			return err
		}
	}
	return nil
}

func validatePlaygroundParameterValue(parameter PlaygroundParameter, value any) error {
	switch parameter.Type {
	case PlaygroundParameterString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter %s must be a string", parameter.Key)
		}
	case PlaygroundParameterBoolean:
		if _, ok := value.(bool); !ok {
			if text, stringValue := value.(string); stringValue {
				if _, err := strconv.ParseBool(text); err == nil {
					return nil
				}
			}
			return fmt.Errorf("parameter %s must be a boolean", parameter.Key)
		}
	case PlaygroundParameterNumber:
		number, err := playgroundNumber(value)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("parameter %s must be a number", parameter.Key)
		}
		if parameter.Min != nil && number < *parameter.Min {
			return fmt.Errorf("parameter %s must be at least %v", parameter.Key, *parameter.Min)
		}
		if parameter.Max != nil && number > *parameter.Max {
			return fmt.Errorf("parameter %s must be at most %v", parameter.Key, *parameter.Max)
		}
	case PlaygroundParameterEnum:
		matched := false
		for _, option := range parameter.Options {
			if reflect.DeepEqual(option, value) || fmt.Sprint(option) == fmt.Sprint(value) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("parameter %s has an unsupported value", parameter.Key)
		}
	}
	return nil
}

func GetPlaygroundParameterValue(values map[string]any, path string) (any, bool) {
	current := any(values)
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func playgroundNumber(value any) (float64, error) {
	switch number := value.(type) {
	case float64:
		return number, nil
	case float32:
		return float64(number), nil
	case int:
		return float64(number), nil
	case string:
		return strconv.ParseFloat(number, 64)
	default:
		return 0, fmt.Errorf("unsupported number type")
	}
}

type ModelPlaygroundConfig struct {
	Capabilities []string                     `json:"capabilities,omitempty"`
	Parameters   []PlaygroundParameter        `json:"parameters,omitempty"`
	Integration  *PlaygroundIntegrationConfig `json:"integration,omitempty"`
}

type ModelCapabilityConfig struct {
	Endpoints map[string]ModelPlaygroundConfig `json:"endpoints"`
}

func ParseModelCapabilityConfig(raw string) (ModelCapabilityConfig, error) {
	config := ModelCapabilityConfig{Endpoints: map[string]ModelPlaygroundConfig{}}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config, nil
	}
	if err := common.UnmarshalJsonStr(raw, &config); err != nil {
		return ModelCapabilityConfig{}, err
	}
	if config.Endpoints == nil {
		config.Endpoints = map[string]ModelPlaygroundConfig{}
	}
	for endpointName, endpoint := range config.Endpoints {
		if strings.TrimSpace(endpointName) == "" {
			return ModelCapabilityConfig{}, fmt.Errorf("endpoint name cannot be empty")
		}
		if err := validateModelPlaygroundConfig(&endpoint); err != nil {
			return ModelCapabilityConfig{}, fmt.Errorf("endpoint %s: %w", endpointName, err)
		}
	}
	return config, nil
}

func (config ModelCapabilityConfig) EndpointConfigs() map[string]ModelEndpointConfig {
	endpoints := make(map[string]ModelEndpointConfig, len(config.Endpoints))
	for endpointName, playground := range config.Endpoints {
		playgroundCopy := playground
		endpoints[endpointName] = ModelEndpointConfig{Playground: &playgroundCopy}
	}
	return endpoints
}

type PlaygroundIntegrationInterface struct {
	Key                string   `json:"key"`
	Title              string   `json:"title"`
	Description        string   `json:"description,omitempty"`
	Method             string   `json:"method"`
	Path               string   `json:"path"`
	RequestDescription string   `json:"request_description,omitempty"`
	CurlTemplate       string   `json:"curl_template"`
	ResponseExample    string   `json:"response_example,omitempty"`
	Notes              []string `json:"notes,omitempty"`
}

type PlaygroundIntegrationConfig struct {
	Overview         string                           `json:"overview,omitempty"`
	DocumentationURL string                           `json:"documentation_url,omitempty"`
	Interfaces       []PlaygroundIntegrationInterface `json:"interfaces"`
	ResultNote       string                           `json:"result_note,omitempty"`
	CompleteExample  string                           `json:"complete_example,omitempty"`
}

type ModelEndpointConfig struct {
	Path       string                 `json:"path,omitempty"`
	Method     string                 `json:"method,omitempty"`
	Playground *ModelPlaygroundConfig `json:"playground,omitempty"`
}

type PlaygroundModelOption struct {
	ModelName              string                         `json:"model_name"`
	SupportedEndpointTypes []constant.EndpointType        `json:"supported_endpoint_types"`
	Endpoints              map[string]ModelEndpointConfig `json:"endpoints"`
}

func GetDefaultModelEndpointConfig(endpointType constant.EndpointType) (ModelEndpointConfig, bool) {
	info, ok := common.GetDefaultEndpointInfo(endpointType)
	if !ok {
		return ModelEndpointConfig{}, false
	}
	return ModelEndpointConfig{Path: info.Path, Method: info.Method}, true
}

func ParseModelEndpointConfigs(raw string) (map[string]ModelEndpointConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]ModelEndpointConfig{}, nil
	}

	var endpoints map[string]any
	if err := common.UnmarshalJsonStr(raw, &endpoints); err != nil {
		var endpointNames []string
		if arrayErr := common.UnmarshalJsonStr(raw, &endpointNames); arrayErr != nil {
			return nil, err
		}
		result := make(map[string]ModelEndpointConfig, len(endpointNames))
		for _, endpointName := range endpointNames {
			if endpointName != "" {
				result[endpointName] = ModelEndpointConfig{}
			}
		}
		return result, nil
	}

	result := make(map[string]ModelEndpointConfig, len(endpoints))
	for endpointName, value := range endpoints {
		config := ModelEndpointConfig{Method: "POST"}
		switch endpointValue := value.(type) {
		case string:
			config.Path = endpointValue
		case map[string]any:
			encoded, err := common.Marshal(endpointValue)
			if err != nil {
				return nil, err
			}
			if err := common.Unmarshal(encoded, &config); err != nil {
				return nil, err
			}
		default:
			continue
		}
		if err := validateModelPlaygroundConfig(config.Playground); err != nil {
			return nil, fmt.Errorf("endpoint %s: %w", endpointName, err)
		}
		result[endpointName] = config
	}
	return result, nil
}

func validateModelPlaygroundConfig(config *ModelPlaygroundConfig) error {
	if config == nil {
		return nil
	}

	for _, capability := range config.Capabilities {
		if _, ok := playgroundCapabilities[capability]; !ok {
			return fmt.Errorf("unsupported playground capability %s", capability)
		}
	}

	seen := make(map[string]struct{}, len(config.Parameters))
	for _, parameter := range config.Parameters {
		parameter.Key = strings.TrimSpace(parameter.Key)
		if parameter.Key == "" {
			return fmt.Errorf("parameter key is required")
		}
		if _, ok := seen[parameter.Key]; ok {
			return fmt.Errorf("duplicate parameter key %s", parameter.Key)
		}
		seen[parameter.Key] = struct{}{}
		if err := validatePlaygroundRequestPath(parameter.RequestPath); err != nil {
			return fmt.Errorf("parameter %s: %w", parameter.Key, err)
		}

		switch parameter.Type {
		case PlaygroundParameterString, PlaygroundParameterBoolean:
		case PlaygroundParameterNumber:
			if parameter.Min != nil && parameter.Max != nil && *parameter.Min > *parameter.Max {
				return fmt.Errorf("parameter %s has min greater than max", parameter.Key)
			}
		case PlaygroundParameterEnum:
			if len(parameter.Options) == 0 {
				return fmt.Errorf("enum parameter %s requires options", parameter.Key)
			}
		default:
			return fmt.Errorf("parameter %s has unsupported type %s", parameter.Key, parameter.Type)
		}
		if parameter.Default != nil && parameter.Default != "" {
			if err := validatePlaygroundParameterDefault(parameter); err != nil {
				return fmt.Errorf("invalid default: %w", err)
			}
		}
	}
	return validatePlaygroundIntegration(config.Integration)
}

var playgroundIntegrationVariablePattern = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)

func validatePlaygroundIntegration(config *PlaygroundIntegrationConfig) error {
	if config == nil {
		return nil
	}
	if len(config.Interfaces) == 0 {
		return fmt.Errorf("integration requires at least one interface")
	}
	if len(config.Interfaces) > 8 {
		return fmt.Errorf("integration supports at most 8 interfaces")
	}
	if len(config.Overview) > 500 || len(config.ResultNote) > 500 {
		return fmt.Errorf("integration text is too long")
	}
	if len(config.CompleteExample) > 20000 {
		return fmt.Errorf("template text is too long")
	}
	if config.DocumentationURL != "" {
		parsed, err := url.Parse(config.DocumentationURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("documentation_url must be an absolute HTTP or HTTPS URL")
		}
	}

	allowedMethods := map[string]struct{}{
		"GET": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {},
	}
	seenKeys := make(map[string]struct{}, len(config.Interfaces))
	for _, item := range config.Interfaces {
		key := strings.TrimSpace(item.Key)
		if key == "" || len(key) > 64 {
			return fmt.Errorf("integration interface key is required and must not exceed 64 characters")
		}
		if _, exists := seenKeys[key]; exists {
			return fmt.Errorf("duplicate interface key %s", key)
		}
		seenKeys[key] = struct{}{}
		if strings.TrimSpace(item.Title) == "" || len(item.Title) > 120 {
			return fmt.Errorf("integration interface %s title is required and must not exceed 120 characters", key)
		}
		method := strings.ToUpper(strings.TrimSpace(item.Method))
		if _, ok := allowedMethods[method]; !ok {
			return fmt.Errorf("integration interface %s has unsupported method %s", key, item.Method)
		}
		if !strings.HasPrefix(strings.TrimSpace(item.Path), "/") {
			return fmt.Errorf("integration interface %s path must start with /", key)
		}
		if strings.TrimSpace(item.CurlTemplate) == "" {
			return fmt.Errorf("integration interface %s curl_template is required", key)
		}
		if len(item.Description) > 500 || len(item.RequestDescription) > 500 {
			return fmt.Errorf("integration interface %s description is too long", key)
		}
		if len(item.CurlTemplate) > 20000 || len(item.ResponseExample) > 20000 {
			return fmt.Errorf("integration interface %s template text is too long", key)
		}
		for _, note := range item.Notes {
			if len(note) > 500 {
				return fmt.Errorf("integration interface %s note is too long", key)
			}
		}
		for _, text := range []string{item.Path, item.CurlTemplate, item.ResponseExample} {
			if err := validatePlaygroundIntegrationVariables(text); err != nil {
				return fmt.Errorf("integration interface %s: %w", key, err)
			}
		}
	}
	return validatePlaygroundIntegrationVariables(config.CompleteExample)
}

func validatePlaygroundIntegrationVariables(value string) error {
	allowed := map[string]struct{}{
		"base_url": {}, "api_key": {}, "model": {}, "group": {},
		"prompt": {}, "parameters_json": {}, "request_json": {}, "request_curl": {}, "reference_url": {}, "task_id": {},
	}
	for _, match := range playgroundIntegrationVariablePattern.FindAllStringSubmatch(value, -1) {
		if _, ok := allowed[match[1]]; !ok {
			return fmt.Errorf("unsupported template variable %s", match[1])
		}
	}
	return nil
}

func validatePlaygroundParameterDefault(parameter PlaygroundParameter) error {
	switch parameter.Type {
	case PlaygroundParameterString:
		if _, ok := parameter.Default.(string); !ok {
			return fmt.Errorf("parameter %s must be a string", parameter.Key)
		}
	case PlaygroundParameterBoolean:
		if _, ok := parameter.Default.(bool); !ok {
			return fmt.Errorf("parameter %s must be a boolean", parameter.Key)
		}
	default:
		return validatePlaygroundParameterValue(parameter, parameter.Default)
	}
	return nil
}

func validatePlaygroundRequestPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return fmt.Errorf("request path contains an empty segment")
		}
		switch part {
		case "__proto__", "prototype", "constructor":
			return fmt.Errorf("request path contains an unsafe segment")
		}
	}
	return nil
}

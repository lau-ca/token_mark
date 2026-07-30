package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelValidateSettingsRejectsUnknownImagePromptPlaceholder(t *testing.T) {
	setting := `{"image_prompt_parameter_append":{"enabled":true,"template":"size={{size}} style={{style}}"}}`
	channel := Channel{Setting: &setting}

	err := channel.ValidateSettings()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image_prompt_parameter_append: unsupported placeholder: style")
}

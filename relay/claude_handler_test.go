package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

func TestShouldClaudeUseResponsesCompatibilityForCodex(t *testing.T) {
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	defer func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	}()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeCodex,
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: false,
			},
		},
	}

	require.True(t, shouldClaudeUseResponsesCompatibility(info))
}

func TestShouldClaudeUseResponsesCompatibilityRejectsPassThroughForCodex(t *testing.T) {
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	defer func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	}()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeCodex,
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: true,
			},
		},
	}

	require.False(t, shouldClaudeUseResponsesCompatibility(info))
}

func TestShouldClaudeUseResponsesCompatibilityUsesExistingPolicyForOtherChannels(t *testing.T) {
	global := model_setting.GetGlobalSettings()
	originalPassThrough := global.PassThroughRequestEnabled
	originalPolicy := global.ChatCompletionsToResponsesPolicy
	global.PassThroughRequestEnabled = false
	global.ChatCompletionsToResponsesPolicy = model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   false,
		ChannelTypes:  []int{constant.ChannelTypeOpenAI},
		ModelPatterns: []string{"^gpt-5.*$"},
	}
	defer func() {
		global.PassThroughRequestEnabled = originalPassThrough
		global.ChatCompletionsToResponsesPolicy = originalPolicy
	}()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   12,
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: false,
			},
		},
		OriginModelName: "gpt-5",
	}

	require.True(t, shouldClaudeUseResponsesCompatibility(info))

	info.OriginModelName = "gpt-4o"
	require.False(t, shouldClaudeUseResponsesCompatibility(info))
}

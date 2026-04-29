package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestForceResponsesStreamForCodexClaude(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeCodex,
		},
	}

	forceResponsesStreamForCodexClaude(info, req)

	require.NotNil(t, req.Stream)
	require.True(t, *req.Stream)
}

func TestForceResponsesStreamForCodexClaudeSkipsOtherFormats(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{Stream: common.GetPointer(false)}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeCodex,
		},
	}

	forceResponsesStreamForCodexClaude(info, req)

	require.NotNil(t, req.Stream)
	require.False(t, *req.Stream)
}

func TestForceResponsesStreamForCodexClaudeSkipsOtherChannels(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{Stream: common.GetPointer(false)}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
	}

	forceResponsesStreamForCodexClaude(info, req)

	require.NotNil(t, req.Stream)
	require.False(t, *req.Stream)
}

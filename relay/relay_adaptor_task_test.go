package relay

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	taskseedance "github.com/QuantumNous/new-api/relay/channel/task/seedance"
	tasksora "github.com/QuantumNous/new-api/relay/channel/task/sora"
	"github.com/stretchr/testify/require"
)

func TestTaskAdaptorRoutingKeepsSeedanceIsolated(t *testing.T) {
	seedance := GetTaskAdaptor(constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeSeedance)))
	require.IsType(t, &taskseedance.TaskAdaptor{}, seedance)

	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeSora} {
		legacy := GetTaskAdaptor(constant.TaskPlatform(fmt.Sprintf("%d", channelType)))
		require.IsType(t, &tasksora.TaskAdaptor{}, legacy)
	}
}

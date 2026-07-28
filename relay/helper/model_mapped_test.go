package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelMappedHelperUsesCompositeBillingModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	request := &dto.ImageRequest{Model: "admin-image-model"}
	info := &relaycommon.RelayInfo{
		OriginModelName:  "admin-image-model",
		BillingModelName: "gpt-image-2-w",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-image-2-w",
		},
	}

	err := ModelMappedHelper(c, info, request)

	require.NoError(t, err)
	assert.Equal(t, "gpt-image-2-w", request.Model)
	assert.Equal(t, "admin-image-model", info.OriginModelName)
}

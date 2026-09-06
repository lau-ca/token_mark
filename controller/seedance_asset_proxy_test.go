package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaydto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSeedanceAssetUpstreamUsesRegisteredDoubaoChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/volc/ark?Action=CreateAsset&Version=2024-01-01", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeDoubaoVideo,
		ChannelBaseUrl: "https://ai-api.example.com/root/",
		ApiKey:         "registered-key",
		ChannelSetting: relaydto.ChannelSettings{Proxy: "http://proxy.example.com:8080"},
	}}

	upstreamURL, key, proxy, err := resolveSeedanceAssetUpstream(context, info)

	require.NoError(t, err)
	assert.Equal(t, "https://ai-api.example.com/root/v1/volc/ark?Action=CreateAsset&Version=2024-01-01", upstreamURL.String())
	assert.Equal(t, "registered-key", key)
	assert.Equal(t, "http://proxy.example.com:8080", proxy)
}

func TestResolveSeedanceAssetUpstreamKeepsLegacyEnvironmentRoute(t *testing.T) {
	t.Setenv("SEEDANCE_ASSET_PROXY_BASE_URL", "https://legacy.example.com/custom/ark")
	t.Setenv("SEEDANCE_ASSET_PROXY_API_KEY", "legacy-key")
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/volc/ark?Action=GetAsset&Version=2024-01-01", nil)

	upstreamURL, key, proxy, err := resolveSeedanceAssetUpstream(context, &relaycommon.RelayInfo{})

	require.NoError(t, err)
	assert.Equal(t, "https://legacy.example.com/custom/ark?Action=GetAsset&Version=2024-01-01", upstreamURL.String())
	assert.Equal(t, "legacy-key", key)
	assert.Empty(t, proxy)
}

func TestSeedanceAssetActionsCoverAllDocumentedMenus(t *testing.T) {
	documentedActions := []string{
		"CreateVisualValidateSession",
		"GetVisualValidateResult",
		"CreateAsset",
		"GetAsset",
		"ListAssets",
		"UpdateAsset",
		"DeleteAsset",
		"CreateAssetGroup",
		"GetAssetGroup",
		"ListAssetGroups",
		"UpdateAssetGroup",
		"DeleteAssetGroup",
	}

	assert.Len(t, seedanceAssetActions, len(documentedActions))
	for _, action := range documentedActions {
		_, exists := seedanceAssetActions[action]
		assert.True(t, exists, action)
	}
}

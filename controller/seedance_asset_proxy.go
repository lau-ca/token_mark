package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

var seedanceAssetActions = map[string]struct{}{
	"CreateAssetGroup":            {},
	"CreateVisualValidateSession": {},
	"GetVisualValidateResult":     {},
	"CreateAsset":                 {},
	"GetAsset":                    {},
	"ListAssets":                  {},
	"UpdateAsset":                 {},
	"DeleteAsset":                 {},
	"GetAssetGroup":               {},
	"ListAssetGroups":             {},
	"UpdateAssetGroup":            {},
	"DeleteAssetGroup":            {},
}

var seedanceAssetHopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

func SeedanceAssetProxy(c *gin.Context) {
	action := strings.TrimSpace(c.Query("Action"))
	if _, ok := seedanceAssetActions[action]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"message": "unsupported asset action",
			"type":    "invalid_request_error",
		}})
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
			"message": "failed to initialize asset billing",
			"type":    "server_error",
		}})
		return
	}
	relayInfo.OriginModelName = constant.SeedanceAssetBillingModel
	relayInfo.BillingModelName = constant.SeedanceAssetBillingModel
	relayInfo.Action = action
	relayInfo.ForcePreConsume = true

	upstreamURL, upstreamAPIKey, upstreamProxy, err := resolveSeedanceAssetUpstream(c, relayInfo)
	if err != nil {
		logger.LogError(c, err.Error())
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"message": "asset upstream is not configured",
			"type":    "server_error",
		}})
		return
	}

	priceData, err := helper.ModelPriceHelperPerCall(c, relayInfo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "invalid_request_error",
		}})
		return
	}
	relayInfo.PriceData = priceData
	if !priceData.FreeModel {
		if billingErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); billingErr != nil {
			c.JSON(billingErr.StatusCode, gin.H{"error": billingErr.ToOpenAIError()})
			return
		}
	}
	settled := false
	defer func() {
		if !settled && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	requestContext, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, upstreamURL.String(), c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
			"message": "failed to create asset upstream request",
			"type":    "server_error",
		}})
		return
	}
	copySeedanceAssetHeaders(request.Header, c.Request.Header)
	request.Header.Set("Authorization", "Bearer "+upstreamAPIKey)

	httpClient, err := service.GetHttpClientWithProxy(upstreamProxy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
			"message": "failed to initialize asset upstream client",
			"type":    "server_error",
		}})
		return
	}
	response, err := httpClient.Do(request)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("seedance asset upstream request failed: %s", err.Error()))
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{
			"message": "asset upstream request failed",
			"type":    "server_error",
		}})
		return
	}
	defer response.Body.Close()

	upstreamSucceeded := response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices
	if upstreamSucceeded && !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "application/json") {
		logger.LogError(c, fmt.Sprintf("seedance asset upstream returned unexpected content type %q", response.Header.Get("Content-Type")))
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{
			"message": "asset upstream returned an invalid response",
			"type":    "server_error",
		}})
		return
	}

	if upstreamSucceeded {
		if err := service.SettleBilling(c, relayInfo, priceData.Quota); err != nil {
			logger.LogError(c, fmt.Sprintf("seedance asset billing settlement failed: %s", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
				"message": "asset billing settlement failed",
				"type":    "server_error",
			}})
			return
		}
		settled = true
		service.LogAssetConsumption(c, relayInfo, action)
	}

	copySeedanceAssetHeaders(c.Writer.Header(), response.Header)
	c.Status(response.StatusCode)
	if _, err := io.Copy(c.Writer, response.Body); err != nil {
		logger.LogError(c, fmt.Sprintf("failed to stream seedance asset response: %s", err.Error()))
	}
}

func resolveSeedanceAssetUpstream(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*url.URL, string, string, error) {
	registeredRoute := c.Request.URL.Path == "/v1/volc/ark"
	baseURL := strings.TrimSpace(os.Getenv("SEEDANCE_ASSET_PROXY_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("SEEDANCE_ASSET_PROXY_API_KEY"))
	proxy := ""
	if registeredRoute {
		if relayInfo == nil || relayInfo.ChannelMeta == nil || relayInfo.ChannelType != constant.ChannelTypeDoubaoVideo {
			return nil, "", "", fmt.Errorf("registered asset route requires a Doubao Video channel")
		}
		baseURL = strings.TrimSpace(relayInfo.ChannelBaseUrl)
		apiKey = strings.TrimSpace(relayInfo.ApiKey)
		proxy = relayInfo.ChannelSetting.Proxy
	}
	upstreamURL, err := url.Parse(baseURL)
	if err != nil || (upstreamURL.Scheme != "http" && upstreamURL.Scheme != "https") || upstreamURL.Host == "" || upstreamURL.User != nil {
		return nil, "", "", fmt.Errorf("invalid seedance asset upstream base URL")
	}
	if registeredRoute {
		if !strings.HasSuffix(strings.TrimRight(upstreamURL.Path, "/"), "/v1/volc/ark") {
			upstreamURL.Path = strings.TrimRight(upstreamURL.Path, "/") + "/v1/volc/ark"
		}
	}
	if apiKey == "" {
		return nil, "", "", fmt.Errorf("seedance asset upstream API key is empty")
	}
	upstreamURL.RawQuery = c.Request.URL.RawQuery
	return upstreamURL, apiKey, proxy, nil
}

func copySeedanceAssetHeaders(destination, source http.Header) {
	for key, values := range source {
		canonicalKey := http.CanonicalHeaderKey(key)
		if canonicalKey == "Authorization" {
			continue
		}
		if _, skip := seedanceAssetHopByHopHeaders[canonicalKey]; skip {
			continue
		}
		for _, value := range values {
			destination.Add(canonicalKey, value)
		}
	}
}

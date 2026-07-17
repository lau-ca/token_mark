package controller

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	balancePlatformNewAPI       = "new_api"
	balancePlatformSub2API      = "sub2api"
	maxBalanceResponseBodyBytes = 1 << 20
)

type newAPIStatusResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		QuotaPerUnit float64 `json:"quota_per_unit"`
	} `json:"data"`
}

type newAPIUserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ID    int   `json:"id"`
		Quota int64 `json:"quota"`
	} `json:"data"`
}

type sub2APIUsageResponse struct {
	Mode    string   `json:"mode"`
	Balance *float64 `json:"balance"`
	Unit    string   `json:"unit"`
}

type OpenAISubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type OpenAIUsageResponse struct {
	Object     string  `json:"object"`
	TotalUsage float64 `json:"total_usage"`
}

func GetAuthHeader(token string) http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	return headers
}

func GetClaudeAuthHeader(token string) http.Header {
	headers := http.Header{}
	headers.Set("x-api-key", token)
	headers.Set("anthropic-version", "2023-06-01")
	return headers
}

func GetResponseBody(method, endpoint string, channel *model.Channel, headers http.Header) ([]byte, error) {
	request, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", response.StatusCode)
	}
	return io.ReadAll(response.Body)
}

func normalizeBalanceBaseURL(rawURL string) (string, error) {
	normalized := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if normalized == "" {
		return "", errors.New("未配置余额查询地址")
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("余额查询地址无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("余额查询地址仅支持 HTTP 或 HTTPS")
	}
	return normalized, nil
}

func getChannelBalanceResponse(channel *model.Channel, endpoint string, headers http.Header) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建余额查询请求失败: %w", err)
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, fmt.Errorf("创建余额查询客户端失败: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("余额查询请求失败: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("余额查询返回异常状态码: %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBalanceResponseBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取余额查询响应失败: %w", err)
	}
	if len(body) > maxBalanceResponseBodyBytes {
		return nil, errors.New("余额查询响应过大")
	}
	return body, nil
}

func balanceAuthHeaders(token string) http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("Accept", "application/json")
	return headers
}

func upstreamBalanceError(prefix, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New(prefix)
	}
	return fmt.Errorf("%s: %s", prefix, message)
}

func queryNewAPIAccountBalance(channel *model.Channel) (float64, error) {
	baseURL, err := normalizeBalanceBaseURL(channel.BalanceBaseURL)
	if err != nil {
		return 0, err
	}
	if channel.BalanceUserID <= 0 {
		return 0, errors.New("未配置 New API 用户 ID")
	}
	authKey := strings.TrimSpace(channel.BalanceAuthKey)
	if authKey == "" {
		return 0, errors.New("未配置 New API 账户访问令牌")
	}

	statusBody, err := getChannelBalanceResponse(channel, baseURL+"/api/status", http.Header{"Accept": []string{"application/json"}})
	if err != nil {
		return 0, fmt.Errorf("获取 New API 额度配置失败: %w", err)
	}
	status := newAPIStatusResponse{}
	if err := common.Unmarshal(statusBody, &status); err != nil {
		return 0, fmt.Errorf("解析 New API 额度配置失败: %w", err)
	}
	if !status.Success {
		return 0, upstreamBalanceError("获取 New API 额度配置失败", status.Message)
	}
	if status.Data.QuotaPerUnit <= 0 || math.IsNaN(status.Data.QuotaPerUnit) || math.IsInf(status.Data.QuotaPerUnit, 0) {
		return 0, errors.New("New API quota_per_unit 无效")
	}

	headers := balanceAuthHeaders(authKey)
	headers.Set("New-Api-User", strconv.Itoa(channel.BalanceUserID))
	userBody, err := getChannelBalanceResponse(channel, baseURL+"/api/user/self", headers)
	if err != nil {
		return 0, fmt.Errorf("获取 New API 账户余额失败: %w", err)
	}
	user := newAPIUserResponse{}
	if err := common.Unmarshal(userBody, &user); err != nil {
		return 0, fmt.Errorf("解析 New API 账户余额失败: %w", err)
	}
	if !user.Success {
		return 0, upstreamBalanceError("获取 New API 账户余额失败", user.Message)
	}
	if user.Data.ID != channel.BalanceUserID {
		return 0, fmt.Errorf("New API 返回的用户 ID 不匹配: %d", user.Data.ID)
	}
	if user.Data.Quota < 0 {
		return 0, errors.New("New API 账户余额无效")
	}

	balance := float64(user.Data.Quota) / status.Data.QuotaPerUnit
	if balance < 0 || math.IsNaN(balance) || math.IsInf(balance, 0) {
		return 0, errors.New("New API 账户余额无效")
	}
	return balance, nil
}

func querySub2APIAccountBalance(channel *model.Channel) (float64, error) {
	baseURL, err := normalizeBalanceBaseURL(channel.BalanceBaseURL)
	if err != nil {
		return 0, err
	}
	apiKey := strings.TrimSpace(channel.Key)
	if apiKey == "" {
		return 0, errors.New("未配置 Sub2API 渠道 API Key")
	}

	body, err := getChannelBalanceResponse(channel, baseURL+"/v1/usage", balanceAuthHeaders(apiKey))
	if err != nil {
		return 0, fmt.Errorf("获取 Sub2API 账户余额失败: %w", err)
	}
	usage := sub2APIUsageResponse{}
	if err := common.Unmarshal(body, &usage); err != nil {
		return 0, fmt.Errorf("解析 Sub2API 账户余额失败: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(usage.Mode), "unrestricted") || usage.Balance == nil {
		return 0, errors.New("Sub2API 未返回钱包余额，Key 限额或订阅额度不能作为账户余额")
	}
	if !strings.EqualFold(strings.TrimSpace(usage.Unit), "USD") {
		return 0, fmt.Errorf("Sub2API 账户余额单位必须为 USD，实际为 %s", usage.Unit)
	}
	if *usage.Balance < 0 || math.IsNaN(*usage.Balance) || math.IsInf(*usage.Balance, 0) {
		return 0, errors.New("Sub2API 账户余额无效")
	}
	return *usage.Balance, nil
}

func updateChannelBalance(channel *model.Channel) (float64, error) {
	var (
		balance float64
		err     error
	)
	switch strings.ToLower(strings.TrimSpace(channel.BalancePlatform)) {
	case balancePlatformNewAPI:
		balance, err = queryNewAPIAccountBalance(channel)
	case balancePlatformSub2API:
		balance, err = querySub2APIAccountBalance(channel)
	case "":
		return 0, errors.New("未配置余额平台类型")
	default:
		return 0, fmt.Errorf("不支持的余额平台类型: %s", channel.BalancePlatform)
	}
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func UpdateChannelBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.CacheGetChannel(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channel.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "多密钥渠道不支持余额查询",
		})
		return
	}
	balance, err := updateChannelBalance(channel)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"balance": balance,
	})
}

func updateAllChannelsBalance() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled || channel.ChannelInfo.IsMultiKey {
			continue
		}
		balance, err := updateChannelBalance(channel)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to update channel balance: channel_id=%d, channel_name=%s, error=%v", channel.Id, channel.Name, err))
			continue
		}
		if balance <= 0 {
			service.DisableChannel(*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan()), "余额不足")
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func UpdateAllChannelsBalance(c *gin.Context) {
	if err := updateAllChannelsBalance(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func AutomaticallyUpdateChannels(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		common.SysLog("updating all channels")
		_ = updateAllChannelsBalance()
		common.SysLog("channels update done")
	}
}

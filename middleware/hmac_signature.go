package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
)

// HMACNonceExpiration Nonce 有效期（与时间戳一致）
const HMACNonceExpiration = 5 * time.Minute

var (
	allowedClientIPs   []net.IP
	allowedClientCIDR  []*net.IPNet
	ipConfigOnce       sync.Once
	ipConfigErr        error
	nonceEnabled       = false // 是否启用 Nonce 防重放
	nonceEnabledOnce   sync.Once
)

// initIPConfig 初始化 IP 配置（只执行一次）
func initIPConfig() {
	allowedIPsStr := os.Getenv("HMAC_ALLOWED_IPS")
	if allowedIPsStr == "" {
		return
	}
	parts := strings.Split(allowedIPsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			_, ipNet, err := net.ParseCIDR(part)
			if err == nil {
				allowedClientCIDR = append(allowedClientCIDR, ipNet)
			}
		} else {
			ip := net.ParseIP(part)
			if ip != nil {
				allowedClientIPs = append(allowedClientIPs, ip)
			}
		}
	}
}

func initNonceConfig() {
	nonceEnabled = os.Getenv("HMAC_NONCE_ENABLED") == "true"
}

// isIPAllowed 检查 IP 是否在允许列表中
func isIPAllowed(ip net.IP) bool {
	ipConfigOnce.Do(initIPConfig)

	if len(allowedClientIPs) == 0 && len(allowedClientCIDR) == 0 {
		return true
	}

	for _, allowed := range allowedClientIPs {
		if ip.Equal(allowed) {
			return true
		}
	}
	for _, cidr := range allowedClientCIDR {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// GetRealClientIP 获取真实客户端 IP
func GetRealClientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		realIP := net.ParseIP(strings.TrimSpace(ips[0]))
		if realIP != nil {
			return realIP.String()
		}
	}

	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		realIP := net.ParseIP(strings.TrimSpace(xri))
		if realIP != nil {
			return realIP.String()
		}
	}

	return c.ClientIP()
}

// checkNonce 检查 Nonce 是否已使用（防重放）
// 返回 error 表示重放攻击
func checkNonce(nonce string) error {
	if nonce == "" {
		if nonceEnabled {
			return errors.New("Nonce is required when HMAC_NONCE_ENABLED=true")
		}
		return nil
	}

	if !nonceEnabled {
		return nil
	}

	// 检查 Redis 是否可用
	if !common.RedisEnabled {
		// Redis 不可用时，跳过 nonce 检查（降级）
		return nil
	}

	key := "hmac_nonce:" + nonce
	ok, err := common.RedisSetNX(key, "1", HMACNonceExpiration)
	if err != nil {
		// Redis 错误时，降级通过（避免影响可用性）
		return nil
	}
	if !ok {
		return errors.New("Replay attack detected")
	}
	return nil
}

// HMACAuth HMAC 签名验证中间件
// 请求头必须包含：
//   - X-Api-Key: API Key
//   - X-Timestamp: 时间戳（Unix 秒）
//   - X-Nonce: 随机字符串（防重放，建议启用）
//   - X-Signature: HMAC-SHA256 签名，格式：hmac-sha256=<hex>
func HMACAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 0. IP 白名单检查
		clientIP := GetRealClientIP(c)
		if !isIPAllowed(net.ParseIP(clientIP)) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "IP not allowed",
			})
			c.Abort()
			return
		}

		// 1. 获取签名所需参数
		apiKey := c.GetHeader("X-Api-Key")
		timestampStr := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		signature := c.GetHeader("X-Signature")

		if apiKey == "" || timestampStr == "" || signature == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Missing required headers: X-Api-Key, X-Timestamp, X-Signature",
			})
			c.Abort()
			return
		}

		// 1.5 Nonce 检查（防重放）
		nonceEnabledOnce.Do(initNonceConfig)
		if err := checkNonce(nonce); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// 2. 验证时间戳
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid timestamp format",
			})
			c.Abort()
			return
		}

		if abs(time.Now().Unix()-timestamp) > 300 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Timestamp expired, must be within 300 seconds",
			})
			c.Abort()
			return
		}

		// 3. 获取请求体
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Failed to read request body",
			})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		// 4. 构造签名内容：timestamp\nnonce\nmethod\npath\nbody
		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path = path + "?" + c.Request.URL.RawQuery
		}
		message := fmt.Sprintf("%d\n%s\n%s\n%s\n%s",
			timestamp,
			nonce,
			c.Request.Method,
			path,
			string(bodyBytes),
		)

		// 5. 获取 Secret
		secret, ok := getHMACSecret(apiKey)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid API key",
			})
			c.Abort()
			return
		}

		// 6. 计算签名
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(message))
		expectedSig := "hmac-sha256=" + hex.EncodeToString(h.Sum(nil))

		// 7. 恒定时间比较
		if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid signature",
			})
			c.Abort()
			return
		}

		c.Set("hmac_api_key", apiKey)
		c.Next()
	}
}

// getHMACSecret 根据 API Key 获取对应的 HMAC Secret
func getHMACSecret(apiKey string) (string, bool) {
	apiKeysStr := os.Getenv("OPEN_TOKEN_API_KEYS")
	if apiKeysStr == "" {
		legacyKey := os.Getenv("OPEN_TOKEN_API_KEY")
		if legacyKey != "" && apiKey == legacyKey {
			return legacyKey, true
		}
		return "", false
	}

	pairs := strings.Split(apiKeysStr, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 && parts[0] == apiKey {
			return parts[1], true
		}
	}
	return "", false
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

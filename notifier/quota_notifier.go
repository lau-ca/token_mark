package notifier

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// TokenQuotaNotification is the payload sent on token quota changes
type TokenQuotaNotification struct {
	Type      string  `json:"type"`
	Timestamp int64   `json:"timestamp"`
	TokenKey string  `json:"token_key"`
	Amount   float64 `json:"amount"`
}

var tokenQuotaNotifierConfig = struct {
	WebhookURL string
	TimeoutMs  int
}{
	WebhookURL: "",
	TimeoutMs:  5000,
}

// SetTokenQuotaNotifierWebhookURL sets the webhook URL. Empty URL disables the notifier.
func SetTokenQuotaNotifierWebhookURL(webhookURL string) {
	tokenQuotaNotifierConfig.WebhookURL = webhookURL
}

// SetTokenQuotaNotifierTimeoutMs sets the webhook timeout in milliseconds.
func SetTokenQuotaNotifierTimeoutMs(timeoutMs int) {
	tokenQuotaNotifierConfig.TimeoutMs = timeoutMs
}

// NotifyTokenQuotaChangeAsync sends an async HTTP POST when token quota is deducted.
// Only triggers when tokenKey is non-empty. Non-blocking, runs in a goroutine via gopool.
func NotifyTokenQuotaChangeAsync(tokenKey string, change int) {
	if tokenQuotaNotifierConfig.WebhookURL == "" || tokenKey == "" || change == 0 {
		return
	}

	quotaDisplayType := operation_setting.GetQuotaDisplayType()
	quota := float64(-change)

	var amount float64

	switch quotaDisplayType {
	case operation_setting.QuotaDisplayTypeUSD:
		amount = quota / common.QuotaPerUnit
	case operation_setting.QuotaDisplayTypeCNY, operation_setting.QuotaDisplayTypeCustom:
		amount = quota / common.QuotaPerUnit * operation_setting.USDExchangeRate
	default: // TOKENS
		amount = quota
	}

	notification := TokenQuotaNotification{
		Type:      "decrease",
		Timestamp: time.Now().UnixMilli(),
		TokenKey:  tokenKey,
		Amount:    amount,
	}

	payload, _ := common.Marshal(notification)
	common.SysLog(fmt.Sprintf("[quota_notifier] sending webhook: %s", payload))

	common.RelayCtxGo(context.Background(), func() {
		_, err := common.Marshal(notification) // reuse outer payload
		if err != nil {
			common.SysError(fmt.Sprintf("[quota_notifier] failed to marshal: %v", err))
			return
		}

		req, err := http.NewRequest(http.MethodPost, tokenQuotaNotifierConfig.WebhookURL, bytes.NewReader(payload))
		if err != nil {
			common.SysError(fmt.Sprintf("[quota_notifier] failed to create request: %v", err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if token := common.GetEnvOrDefaultString("OPEN_TOKEN_API_KEY", ""); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		client := &http.Client{
			Timeout: time.Duration(tokenQuotaNotifierConfig.TimeoutMs) * time.Millisecond,
		}

		resp, err := client.Do(req)
		if err != nil {
			common.SysError(fmt.Sprintf("[quota_notifier] failed to send: %v", err))
			return
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			common.SysError(fmt.Sprintf("[quota_notifier] non-2xx status: %d", resp.StatusCode))
		}
	})
}

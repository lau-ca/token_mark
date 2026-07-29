package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfiniBaseURL(t *testing.T) {
	assert.Equal(t, infiniSandboxBaseURL, InfiniBaseURL(true))
	assert.Equal(t, infiniProductionBaseURL, InfiniBaseURL(false))
}

func TestInfiniClientCreateOrderSignsRequest(t *testing.T) {
	fixedTime := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, infiniCreateOrderPath, r.URL.Path)
		assert.Equal(t, fixedTime.Format(http.TimeFormat), r.Header.Get("Date"))
		assert.NotEmpty(t, r.Header.Get("Digest"))
		assert.Contains(t, r.Header.Get("Authorization"), `keyId="key-id"`)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"client_reference":"INF-1"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"order_id":"ord-1","request_id":"req-1","checkout_url":"https://checkout.infini.money/pay/1","client_reference":"INF-1"}}`))
	}))
	defer server.Close()

	client := &InfiniClient{
		BaseURL:    server.URL,
		KeyID:      "key-id",
		SecretKey:  "secret",
		HTTPClient: server.Client(),
		Now:        func() time.Time { return fixedTime },
	}
	result, err := client.CreateOrder(context.Background(), &InfiniCreateOrderRequest{
		Amount:          "10.00",
		RequestID:       "req-1",
		ClientReference: "INF-1",
		Currency:        "USD",
		PayMethods:      []int{1},
	})
	require.NoError(t, err)
	assert.Equal(t, "ord-1", result.OrderID)
	assert.Equal(t, "https://checkout.infini.money/pay/1", result.CheckoutURL)
}

func TestVerifyInfiniWebhook(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	timestamp := "1800000000"
	eventID := "evt-1"
	body := []byte(`{"event":"order.completed","status":"paid"}`)
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, _ = mac.Write([]byte(timestamp + "." + eventID + "." + string(body)))
	signature := hex.EncodeToString(mac.Sum(nil))

	require.NoError(t, VerifyInfiniWebhook("webhook-secret", timestamp, eventID, signature, body, now))
	assert.Error(t, VerifyInfiniWebhook("webhook-secret", timestamp, eventID, "bad", body, now))
	assert.Error(t, VerifyInfiniWebhook("webhook-secret", "1799999000", eventID, signature, body, now))
}

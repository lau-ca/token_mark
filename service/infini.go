package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	infiniProductionBaseURL = "https://openapi.infini.money"
	infiniSandboxBaseURL    = "https://openapi-sandbox.infini.money"
	infiniCreateOrderPath   = "/v1/acquiring/order"
	infiniWebhookMaxSkew    = 5 * time.Minute
)

type InfiniClient struct {
	BaseURL    string
	KeyID      string
	SecretKey  string
	HTTPClient *http.Client
	Now        func() time.Time
}

type InfiniCreateOrderRequest struct {
	Amount           string `json:"amount"`
	RequestID        string `json:"request_id"`
	ClientReference  string `json:"client_reference"`
	OrderDescription string `json:"order_desc,omitempty"`
	ExpiresIn        int64  `json:"expires_in,omitempty"`
	MerchantAlias    string `json:"merchant_alias,omitempty"`
	SuccessURL       string `json:"success_url,omitempty"`
	FailureURL       string `json:"failure_url,omitempty"`
	PayMethods       []int  `json:"pay_methods,omitempty"`
	Email            string `json:"email,omitempty"`
	Currency         string `json:"currency,omitempty"`
}

type InfiniCreateOrderResponse struct {
	OrderID         string `json:"order_id"`
	RequestID       string `json:"request_id"`
	CheckoutURL     string `json:"checkout_url"`
	ClientReference string `json:"client_reference"`
}

type InfiniWebhookPayload struct {
	Event            string `json:"event"`
	OrderID          string `json:"order_id"`
	ClientReference  string `json:"client_reference"`
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	Status           string `json:"status"`
	AmountConfirming string `json:"amount_confirming"`
	AmountConfirmed  string `json:"amount_confirmed"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type infiniResponseEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func InfiniBaseURL(sandbox bool) string {
	if sandbox {
		return infiniSandboxBaseURL
	}
	return infiniProductionBaseURL
}

func (client *InfiniClient) CreateOrder(ctx context.Context, payload *InfiniCreateOrderRequest) (*InfiniCreateOrderResponse, error) {
	if client == nil || strings.TrimSpace(client.KeyID) == "" || strings.TrimSpace(client.SecretKey) == "" {
		return nil, errors.New("Infini credentials are not configured")
	}
	if payload == nil {
		return nil, errors.New("Infini order payload is required")
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Infini order: %w", err)
	}

	now := time.Now
	if client.Now != nil {
		now = client.Now
	}
	date := now().UTC().Format(http.TimeFormat)
	headers := client.signHeaders(http.MethodPost, infiniCreateOrderPath, date, body)
	baseURL := strings.TrimRight(client.BaseURL, "/")
	if baseURL == "" {
		baseURL = infiniProductionBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+infiniCreateOrderPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Infini request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Infini order: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Infini response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Infini returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	result := &InfiniCreateOrderResponse{}
	if err := common.Unmarshal(responseBody, result); err == nil && result.CheckoutURL != "" {
		return result, nil
	}
	var envelope infiniResponseEnvelope
	if err := common.Unmarshal(responseBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode Infini response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("Infini returned code %d: %s", envelope.Code, envelope.Message)
	}
	if len(envelope.Data) == 0 {
		return nil, errors.New("Infini response data is empty")
	}
	if err := common.Unmarshal(envelope.Data, result); err != nil {
		return nil, fmt.Errorf("decode Infini order data: %w", err)
	}
	if strings.TrimSpace(result.CheckoutURL) == "" || strings.TrimSpace(result.OrderID) == "" {
		return nil, errors.New("Infini response is missing order_id or checkout_url")
	}
	return result, nil
}

func (client *InfiniClient) signHeaders(method, path, date string, body []byte) map[string]string {
	signingString := fmt.Sprintf("%s\n%s %s\ndate: %s\n", client.KeyID, strings.ToUpper(method), path, date)
	mac := hmac.New(sha256.New, []byte(client.SecretKey))
	_, _ = mac.Write([]byte(signingString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	bodyDigest := sha256.Sum256(body)
	return map[string]string{
		"Date":   date,
		"Digest": "SHA-256=" + base64.StdEncoding.EncodeToString(bodyDigest[:]),
		"Authorization": fmt.Sprintf(
			`Signature keyId="%s",algorithm="hmac-sha256",headers="@request-target date",signature="%s"`,
			client.KeyID,
			signature,
		),
	}
}

func VerifyInfiniWebhook(secret, timestamp, eventID, signature string, body []byte, now time.Time) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("Infini Webhook secret is not configured")
	}
	if timestamp == "" || eventID == "" || signature == "" {
		return errors.New("Infini Webhook headers are incomplete")
	}
	timestampUnix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("Infini Webhook timestamp is invalid")
	}
	webhookTime := time.Unix(timestampUnix, 0)
	if now.Sub(webhookTime) > infiniWebhookMaxSkew || webhookTime.Sub(now) > infiniWebhookMaxSkew {
		return errors.New("Infini Webhook timestamp is outside the allowed window")
	}

	signingContent := timestamp + "." + eventID + "." + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingContent))
	expected := mac.Sum(nil)
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil || !hmac.Equal(expected, provided) {
		return errors.New("Infini Webhook signature is invalid")
	}
	return nil
}

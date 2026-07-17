package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryNewAPIAccountBalance(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer account-token", r.Header.Get("Authorization"))
		assert.Equal(t, "1787", r.Header.Get("New-Api-User"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":1787,"quota":599355309}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	channel := &model.Channel{
		BalanceBaseURL: server.URL + "/",
		BalanceUserID:  1787,
		BalanceAuthKey: "account-token",
	}

	balance, err := queryNewAPIAccountBalance(channel)
	require.NoError(t, err)
	assert.InDelta(t, 1198.710618, balance, 1e-9)
}

func TestQueryNewAPIAccountBalanceRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name         string
		statusBody   string
		accountBody  string
		userID       int
		authKey      string
		wantContains string
	}{
		{name: "missing user id", statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, userID: 0, authKey: "token", wantContains: "用户 ID"},
		{name: "missing auth key", statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, userID: 1, wantContains: "访问令牌"},
		{name: "zero quota unit", statusBody: `{"success":true,"data":{"quota_per_unit":0}}`, accountBody: `{"success":true,"data":{"id":1,"quota":1}}`, userID: 1, authKey: "token", wantContains: "quota_per_unit"},
		{name: "status unsuccessful", statusBody: `{"success":false,"message":"disabled"}`, userID: 1, authKey: "token", wantContains: "disabled"},
		{name: "mismatched user", statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, accountBody: `{"success":true,"data":{"id":2,"quota":1}}`, userID: 1, authKey: "token", wantContains: "用户 ID 不匹配"},
		{name: "negative quota", statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, accountBody: `{"success":true,"data":{"id":1,"quota":-1}}`, userID: 1, authKey: "token", wantContains: "余额无效"},
		{name: "account unsuccessful", statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, accountBody: `{"success":false,"message":"invalid token"}`, userID: 1, authKey: "token", wantContains: "invalid token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.statusBody))
			})
			mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.accountBody))
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			_, err := queryNewAPIAccountBalance(&model.Channel{
				BalanceBaseURL: server.URL,
				BalanceUserID:  tt.userID,
				BalanceAuthKey: tt.authKey,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantContains)
		})
	}
}

func TestQuerySub2APIAccountBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/usage", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(`{"balance":51.6,"isValid":true,"mode":"unrestricted","remaining":51.6,"unit":"USD"}`))
	}))
	defer server.Close()

	balance, err := querySub2APIAccountBalance(&model.Channel{
		Key:            "sk-test",
		BalanceBaseURL: server.URL + "/",
	})
	require.NoError(t, err)
	assert.Equal(t, 51.6, balance)
}

func TestQuerySub2APIAccountBalanceRejectsNonWalletResponses(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		key          string
		wantContains string
	}{
		{name: "missing key", body: `{}`, wantContains: "API Key"},
		{name: "quota limited", key: "sk-test", body: `{"mode":"quota_limited","quota":{"remaining":80},"remaining":80,"unit":"USD"}`, wantContains: "钱包余额"},
		{name: "subscription only", key: "sk-test", body: `{"mode":"unrestricted","subscription":{"daily_limit_usd":60},"remaining":50,"unit":"USD"}`, wantContains: "钱包余额"},
		{name: "non usd", key: "sk-test", body: `{"mode":"unrestricted","balance":50,"unit":"CNY"}`, wantContains: "USD"},
		{name: "negative balance", key: "sk-test", body: `{"mode":"unrestricted","balance":-1,"unit":"USD"}`, wantContains: "余额无效"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			_, err := querySub2APIAccountBalance(&model.Channel{
				Key:            tt.key,
				BalanceBaseURL: server.URL,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantContains)
		})
	}
}

func TestAccountBalanceQueriesRejectInvalidBaseURLAndHTTPStatus(t *testing.T) {
	_, err := queryNewAPIAccountBalance(&model.Channel{
		BalanceBaseURL: "ftp://example.com",
		BalanceUserID:  1,
		BalanceAuthKey: "token",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err = querySub2APIAccountBalance(&model.Channel{
		Key:            "sk-test",
		BalanceBaseURL: server.URL,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

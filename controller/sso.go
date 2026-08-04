package controller

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ssoCodePrefix = "sso:code:"
	ssoCodeTTL    = 5 * time.Minute
)

type ssoIssueRequest struct {
	Client   string `json:"client"`
	ReturnTo string `json:"return_to"`
}

type ssoExchangeRequest struct {
	Code string `json:"code"`
}

type ssoCodePayload struct {
	UserID   int    `json:"user_id"`
	Client   string `json:"client"`
	ReturnTo string `json:"return_to"`
	Expires  int64  `json:"expires"`
}

type ssoUserPayload struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Role        int    `json:"role"`
	Status      int    `json:"status"`
	Email       string `json:"email,omitempty"`
	Group       string `json:"group,omitempty"`
	Quota       int    `json:"quota,omitempty"`
	UsedQuota   int    `json:"used_quota,omitempty"`
}

type ssoIssueResult struct {
	RedirectURL string
	ExpiresAt   int64
}

var (
	ssoMemoryStore     = map[string]ssoCodePayload{}
	ssoMemoryMu        sync.Mutex
	errSSOUserDisabled = errors.New("sso user disabled")
)

func IssueSSOCode(c *gin.Context) {
	var req ssoIssueRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.Client != "image" || req.ReturnTo == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := common.ValidateRedirectURL(req.ReturnTo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgAuthNotLoggedIn),
		})
		return
	}

	result, err := issueSSORedirectURL(userID, req.Client, req.ReturnTo)
	if err != nil {
		if errors.Is(err, errSSOUserDisabled) {
			common.ApiErrorI18n(c, i18n.MsgAuthUserBanned)
			return
		}
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"redirect_url": result.RedirectURL,
			"expires_at":   result.ExpiresAt,
		},
	})
}

func ExchangeSSOCode(c *gin.Context) {
	var req ssoExchangeRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.Code == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	payload, err := consumeSSOCode(req.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if payload.Client != "image" || payload.Expires < time.Now().Unix() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "invalid or expired sso code",
		})
		return
	}

	user, err := model.GetUserById(payload.UserID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Status != common.UserStatusEnabled {
		common.ApiErrorI18n(c, i18n.MsgAuthUserBanned)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": ssoUserPayload{
			Id:          user.Id,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Role:        user.Role,
			Status:      user.Status,
			Email:       user.Email,
			Group:       user.Group,
			Quota:       user.Quota,
			UsedQuota:   user.UsedQuota,
		},
	})
}

func GetSSOSession(c *gin.Context) {
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgAuthNotLoggedIn),
		})
		return
	}

	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Status != common.UserStatusEnabled {
		common.ApiErrorI18n(c, i18n.MsgAuthUserBanned)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": ssoUserPayload{
			Id:          user.Id,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Role:        user.Role,
			Status:      user.Status,
			Email:       user.Email,
			Group:       user.Group,
			Quota:       user.Quota,
			UsedQuota:   user.UsedQuota,
		},
	})
}

func issueSSORedirectURL(userID int, client string, returnTo string) (ssoIssueResult, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return ssoIssueResult{}, err
	}
	if user.Status != common.UserStatusEnabled {
		return ssoIssueResult{}, errSSOUserDisabled
	}

	code, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return ssoIssueResult{}, err
	}

	payload := ssoCodePayload{
		UserID:   user.Id,
		Client:   client,
		ReturnTo: returnTo,
		Expires:  time.Now().Add(ssoCodeTTL).Unix(),
	}
	if err := saveSSOCode(code, payload); err != nil {
		return ssoIssueResult{}, err
	}

	redirectURL, err := appendCode(returnTo, code)
	if err != nil {
		return ssoIssueResult{}, err
	}

	return ssoIssueResult{
		RedirectURL: redirectURL,
		ExpiresAt:   payload.Expires,
	}, nil
}

func saveSSOCode(code string, payload ssoCodePayload) error {
	if common.RedisEnabled && common.RDB != nil {
		data, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		return common.RedisSet(ssoCodePrefix+code, string(data), ssoCodeTTL)
	}

	ssoMemoryMu.Lock()
	defer ssoMemoryMu.Unlock()
	pruneExpiredSSOCodesLocked()
	ssoMemoryStore[code] = payload
	return nil
}

func consumeSSOCode(code string) (ssoCodePayload, error) {
	if common.RedisEnabled && common.RDB != nil {
		key := ssoCodePrefix + code
		ctx := context.Background()
		data, err := common.RDB.GetDel(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return ssoCodePayload{}, errors.New("invalid or expired sso code")
			}
			return ssoCodePayload{}, err
		}
		var payload ssoCodePayload
		if err := common.UnmarshalJsonStr(data, &payload); err != nil {
			return ssoCodePayload{}, err
		}
		return payload, nil
	}

	ssoMemoryMu.Lock()
	defer ssoMemoryMu.Unlock()
	pruneExpiredSSOCodesLocked()
	payload, ok := ssoMemoryStore[code]
	if !ok {
		return ssoCodePayload{}, errors.New("invalid or expired sso code")
	}
	delete(ssoMemoryStore, code)
	return payload, nil
}

func pruneExpiredSSOCodesLocked() {
	now := time.Now().Unix()
	for code, payload := range ssoMemoryStore {
		if payload.Expires < now {
			delete(ssoMemoryStore, code)
		}
	}
}

func appendCode(returnTo string, code string) (string, error) {
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("code", code)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

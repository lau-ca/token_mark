package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const maxCodexRTImportEntries = 100

var codexRTImportMu sync.Mutex

type codexRTImportRequest struct {
	Credentials   string   `json:"credentials"`
	RefreshTokens []string `json:"refresh_tokens"`
	Name          string   `json:"name"`
	NamePrefix    string   `json:"name_prefix"`
	Group         string   `json:"group"`
	Models        string   `json:"models"`
	BaseURL       string   `json:"base_url"`
	Proxy         string   `json:"proxy"`
}

type codexRTExchangeRequest struct {
	RefreshToken string `json:"refresh_token"`
	Credentials  string `json:"credentials"`
	Proxy        string `json:"proxy"`
}

type codexRTImportEntry struct {
	RefreshToken string
	Name         string
	Email        string
	AccountID    string
}

type codexRTImportItemResult struct {
	Index     int    `json:"index"`
	Status    string `json:"status"`
	ChannelID int    `json:"channel_id,omitempty"`
	Name      string `json:"name,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	Email     string `json:"email,omitempty"`
	Error     string `json:"error,omitempty"`
}

type codexRTImportExistingChannel struct {
	ID        int
	Name      string
	Status    int
	AccountID string
	Email     string
	Setting   string
}

type codexRTImportDuplicateIndex struct {
	byAccount map[string]codexRTImportExistingChannel
	byEmail   map[string]codexRTImportExistingChannel
}

func ImportCodexRefreshTokens(c *gin.Context) {
	req := codexRTImportRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	entries, err := parseCodexRTImportEntries(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if len(entries) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请至少提供一个 refresh_token"})
		return
	}
	if len(entries) > maxCodexRTImportEntries {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("单次最多导入 %d 个 Codex RT", maxCodexRTImportEntries),
		})
		return
	}

	requestedName := strings.TrimSpace(req.Name)
	if requestedName != "" && len(entries) == 1 && entries[0].Name == "" {
		entries[0].Name = requestedName
	}
	namePrefix := strings.TrimSpace(req.NamePrefix)
	if namePrefix == "" && requestedName != "" {
		namePrefix = requestedName
	}
	if namePrefix == "" {
		namePrefix = "Codex"
	}
	group := normalizeCodexRTImportCommaList(req.Group, []string{"default"})
	models := normalizeCodexRTImportCommaList(req.Models, codex.ModelList)
	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeCodex]
	}
	proxy := strings.TrimSpace(req.Proxy)

	results := make([]codexRTImportItemResult, 0, len(entries))
	created := 0
	updated := 0
	skipped := 0
	failed := 0

	for i, entry := range entries {
		result := importSingleCodexRefreshToken(c.Request.Context(), i+1, entry, codexRTImportCreateOptions{
			NamePrefix: namePrefix,
			Group:      group,
			Models:     models,
			BaseURL:    baseURL,
			Proxy:      proxy,
		})
		switch result.Status {
		case "created":
			created++
		case "updated":
			updated++
		case "skipped":
			skipped++
		default:
			failed++
		}
		results = append(results, result)
	}

	if created > 0 || updated > 0 {
		model.InitChannelCache()
		service.ResetProxyClientCache()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"total":   len(results),
			"created": created,
			"updated": updated,
			"skipped": skipped,
			"failed":  failed,
			"items":   results,
		},
	})
}

func ExchangeCodexRefreshToken(c *gin.Context) {
	req := codexRTExchangeRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	importReq := codexRTImportRequest{
		Credentials: req.Credentials,
	}
	if refreshToken != "" {
		importReq.RefreshTokens = []string{refreshToken}
	}
	entries, err := parseCodexRTImportEntries(importReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if len(entries) != 1 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请提供且仅提供一个 refresh_token"})
		return
	}

	key, err := buildCodexOAuthKeyFromRefreshToken(c.Request.Context(), entries[0], strings.TrimSpace(req.Proxy))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	encoded, err := common.Marshal(key)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"key":          string(encoded),
			"account_id":   key.AccountID,
			"email":        key.Email,
			"expires_at":   key.Expired,
			"last_refresh": key.LastRefresh,
		},
	})
}

type codexRTImportCreateOptions struct {
	NamePrefix string
	Group      string
	Models     string
	BaseURL    string
	Proxy      string
}

func importSingleCodexRefreshToken(ctx context.Context, index int, entry codexRTImportEntry, opts codexRTImportCreateOptions) codexRTImportItemResult {
	key, err := buildCodexOAuthKeyFromRefreshToken(ctx, entry, opts.Proxy)
	if err != nil {
		return codexRTImportItemResult{Index: index, Status: "failed", Error: err.Error()}
	}
	encoded, err := common.Marshal(key)
	if err != nil {
		return codexRTImportItemResult{Index: index, Status: "failed", AccountID: key.AccountID, Email: key.Email, Error: err.Error()}
	}

	name := buildCodexRTImportChannelName(opts.NamePrefix, entry.Name, key.Email, key.AccountID)
	channel := model.Channel{
		Type:        constant.ChannelTypeCodex,
		Key:         string(encoded),
		Status:      common.ChannelStatusEnabled,
		Name:        name,
		Weight:      common.GetPointer[uint](0),
		CreatedTime: common.GetTimestamp(),
		BaseURL:     common.GetPointer(opts.BaseURL),
		Models:      opts.Models,
		Group:       opts.Group,
		Priority:    common.GetPointer[int64](0),
		AutoBan:     common.GetPointer[int](1),
	}
	if opts.Proxy != "" {
		channel.SetSetting(dto.ChannelSettings{Proxy: opts.Proxy})
	}
	if err := validateChannel(&channel, true); err != nil {
		return codexRTImportItemResult{Index: index, Status: "failed", AccountID: key.AccountID, Email: key.Email, Error: err.Error()}
	}

	codexRTImportMu.Lock()
	defer codexRTImportMu.Unlock()

	duplicateIndex, err := loadCodexRTImportDuplicateIndex()
	if err != nil {
		return codexRTImportItemResult{Index: index, Status: "failed", AccountID: key.AccountID, Email: key.Email, Error: err.Error()}
	}
	if existing, ok := duplicateIndex.find(key.AccountID, key.Email); ok {
		if err := updateCodexRTImportExistingChannel(existing, string(encoded), opts.Proxy); err != nil {
			return codexRTImportItemResult{Index: index, Status: "failed", AccountID: key.AccountID, Email: key.Email, Error: err.Error()}
		}
		return codexRTImportItemResult{
			Index:     index,
			Status:    "updated",
			ChannelID: existing.ID,
			Name:      existing.Name,
			AccountID: key.AccountID,
			Email:     key.Email,
			Error:     fmt.Sprintf("Codex 账号已存在，已更新渠道 #%d 的凭证", existing.ID),
		}
	}

	if err := insertCodexRTImportChannel(&channel); err != nil {
		return codexRTImportItemResult{Index: index, Status: "failed", AccountID: key.AccountID, Email: key.Email, Error: err.Error()}
	}

	return codexRTImportItemResult{
		Index:     index,
		Status:    "created",
		ChannelID: channel.Id,
		Name:      channel.Name,
		AccountID: key.AccountID,
		Email:     key.Email,
	}
}

func buildCodexOAuthKeyFromRefreshToken(ctx context.Context, entry codexRTImportEntry, proxy string) (codex.OAuthKey, error) {
	rt := strings.TrimSpace(entry.RefreshToken)
	if rt == "" {
		return codex.OAuthKey{}, fmt.Errorf("refresh_token 为空")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	tokenRes, err := service.RefreshCodexOAuthTokenWithProxy(refreshCtx, rt, proxy)
	if err != nil {
		return codex.OAuthKey{}, fmt.Errorf("刷新 RT 失败: %w", err)
	}

	accountID, ok := service.ExtractCodexAccountIDFromJWT(tokenRes.AccessToken)
	hintAccountID := strings.TrimSpace(entry.AccountID)
	if ok && hintAccountID != "" && hintAccountID != accountID {
		return codex.OAuthKey{}, fmt.Errorf("RT 返回的 account_id 与输入元数据不一致")
	}
	if !ok {
		accountID = hintAccountID
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return codex.OAuthKey{}, fmt.Errorf("无法从 access_token 提取 account_id")
	}

	email, _ := service.ExtractEmailFromJWT(tokenRes.AccessToken)
	if strings.TrimSpace(email) == "" {
		email = strings.TrimSpace(entry.Email)
	}

	return codex.OAuthKey{
		AccessToken:  tokenRes.AccessToken,
		RefreshToken: tokenRes.RefreshToken,
		AccountID:    accountID,
		LastRefresh:  time.Now().Format(time.RFC3339),
		Expired:      tokenRes.ExpiresAt.Format(time.RFC3339),
		Email:        strings.TrimSpace(email),
		Type:         "codex",
	}, nil
}

func insertCodexRTImportChannel(channel *model.Channel) error {
	tx := model.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Create(channel).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := channel.AddAbilities(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func updateCodexRTImportExistingChannel(existing codexRTImportExistingChannel, key string, proxy string) error {
	updates := map[string]any{
		"key": key,
	}

	if strings.TrimSpace(proxy) != "" {
		setting := dto.ChannelSettings{}
		if strings.TrimSpace(existing.Setting) != "" {
			if err := common.Unmarshal([]byte(existing.Setting), &setting); err != nil {
				return err
			}
		}
		setting.Proxy = strings.TrimSpace(proxy)
		encoded, err := common.Marshal(setting)
		if err != nil {
			return err
		}
		updates["setting"] = string(encoded)
	}

	return model.DB.Model(&model.Channel{}).Where("id = ?", existing.ID).Updates(updates).Error
}

func loadCodexRTImportDuplicateIndex() (*codexRTImportDuplicateIndex, error) {
	index := &codexRTImportDuplicateIndex{
		byAccount: make(map[string]codexRTImportExistingChannel),
		byEmail:   make(map[string]codexRTImportExistingChannel),
	}

	channels := make([]model.Channel, 0)
	if err := model.DB.Select("id,name,status,key,setting").Where("type = ?", constant.ChannelTypeCodex).Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		rawKey := strings.TrimSpace(channel.Key)
		if !strings.HasPrefix(rawKey, "{") {
			continue
		}
		oauthKey, err := codex.ParseOAuthKey(rawKey)
		if err != nil {
			continue
		}
		item := codexRTImportExistingChannel{
			ID:        channel.Id,
			Name:      channel.Name,
			Status:    channel.Status,
			AccountID: strings.TrimSpace(oauthKey.AccountID),
			Email:     strings.TrimSpace(oauthKey.Email),
		}
		if channel.Setting != nil {
			item.Setting = *channel.Setting
		}
		index.add(item)
	}

	return index, nil
}

func (index *codexRTImportDuplicateIndex) add(item codexRTImportExistingChannel) {
	if index == nil {
		return
	}
	if accountKey := normalizeCodexRTImportAccountKey(item.AccountID); accountKey != "" {
		index.byAccount[accountKey] = item
	}
	if emailKey := normalizeCodexRTImportEmailKey(item.Email); emailKey != "" {
		index.byEmail[emailKey] = item
	}
}

func (index *codexRTImportDuplicateIndex) find(accountID string, email string) (codexRTImportExistingChannel, bool) {
	if index == nil {
		return codexRTImportExistingChannel{}, false
	}
	if accountKey := normalizeCodexRTImportAccountKey(accountID); accountKey != "" {
		if existing, ok := index.byAccount[accountKey]; ok {
			return existing, true
		}
	}
	if emailKey := normalizeCodexRTImportEmailKey(email); emailKey != "" {
		if existing, ok := index.byEmail[emailKey]; ok {
			return existing, true
		}
	}
	return codexRTImportExistingChannel{}, false
}

func parseCodexRTImportEntries(req codexRTImportRequest) ([]codexRTImportEntry, error) {
	entries := make([]codexRTImportEntry, 0, len(req.RefreshTokens))
	for _, token := range req.RefreshTokens {
		if entry := normalizeCodexRTImportEntry(codexRTImportEntry{RefreshToken: token}); entry.RefreshToken != "" {
			entries = append(entries, entry)
		}
	}

	raw := strings.TrimSpace(req.Credentials)
	if raw == "" {
		return entries, nil
	}

	if strings.HasPrefix(raw, "[") {
		var values []any
		if err := common.Unmarshal([]byte(raw), &values); err != nil {
			return nil, fmt.Errorf("JSON 数组解析失败: %w", err)
		}
		for i, value := range values {
			entry, err := parseCodexRTImportEntryValue(value)
			if err != nil {
				return nil, fmt.Errorf("第 %d 条凭据解析失败: %w", i+1, err)
			}
			if entry.RefreshToken != "" {
				entries = append(entries, entry)
			}
		}
		return entries, nil
	}

	if strings.HasPrefix(raw, "{") {
		var value map[string]any
		if err := common.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("JSON 对象解析失败: %w", err)
		}
		entry, err := parseCodexRTImportEntryValue(value)
		if err != nil {
			return nil, err
		}
		if entry.RefreshToken != "" {
			entries = append(entries, entry)
		}
		return entries, nil
	}

	for i, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var value map[string]any
			if err := common.Unmarshal([]byte(line), &value); err != nil {
				return nil, fmt.Errorf("第 %d 行 JSON 解析失败: %w", i+1, err)
			}
			entry, err := parseCodexRTImportEntryValue(value)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行凭据解析失败: %w", i+1, err)
			}
			if entry.RefreshToken != "" {
				entries = append(entries, entry)
			}
			continue
		}
		entries = append(entries, normalizeCodexRTImportEntry(codexRTImportEntry{RefreshToken: line}))
	}

	return entries, nil
}

func parseCodexRTImportEntryValue(value any) (codexRTImportEntry, error) {
	switch v := value.(type) {
	case string:
		entry := normalizeCodexRTImportEntry(codexRTImportEntry{RefreshToken: v})
		if entry.RefreshToken == "" {
			return entry, fmt.Errorf("refresh_token 为空")
		}
		return entry, nil
	case map[string]any:
		entry := normalizeCodexRTImportEntry(codexRTImportEntry{
			RefreshToken: firstCodexRTImportString(v, "refresh_token", "refreshToken", "rt", "token"),
			Name:         firstCodexRTImportString(v, "name", "channel_name", "channelName"),
			Email:        firstCodexRTImportString(v, "email"),
			AccountID:    firstCodexRTImportString(v, "account_id", "accountId", "chatgpt_account_id", "chatgptAccountId"),
		})
		if entry.RefreshToken == "" {
			return entry, fmt.Errorf("refresh_token 为空")
		}
		return entry, nil
	default:
		return codexRTImportEntry{}, fmt.Errorf("只支持字符串或 JSON 对象")
	}
}

func normalizeCodexRTImportEntry(entry codexRTImportEntry) codexRTImportEntry {
	entry.RefreshToken = strings.Trim(strings.TrimSpace(entry.RefreshToken), `"`)
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Email = strings.TrimSpace(entry.Email)
	entry.AccountID = strings.TrimSpace(entry.AccountID)
	return entry
}

func firstCodexRTImportString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		default:
			if s := strings.TrimSpace(fmt.Sprintf("%v", v)); s != "" {
				return s
			}
		}
	}
	return ""
}

func normalizeCodexRTImportCommaList(value string, fallback []string) string {
	items := make([]string, 0)
	if strings.TrimSpace(value) != "" {
		items = strings.Split(value, ",")
	} else {
		items = fallback
	}
	seen := make(map[string]struct{}, len(items))
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return strings.Join(normalized, ",")
}

func buildCodexRTImportChannelName(prefix string, requestedName string, email string, accountID string) string {
	if name := strings.TrimSpace(requestedName); name != "" {
		return name
	}
	if email = strings.TrimSpace(email); email != "" {
		return fmt.Sprintf("%s %s", prefix, email)
	}
	accountID = strings.TrimSpace(accountID)
	if len(accountID) > 8 {
		accountID = accountID[len(accountID)-8:]
	}
	if accountID == "" {
		return prefix
	}
	return fmt.Sprintf("%s %s", prefix, accountID)
}

func normalizeCodexRTImportAccountKey(value string) string {
	return strings.TrimSpace(value)
}

func normalizeCodexRTImportEmailKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

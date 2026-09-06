# Channel Account Balance Query Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace channel API-key remaining-quota refreshes with account wallet balance queries for explicitly configured New API and Sub2API upstreams.

**Architecture:** Add dedicated channel configuration columns, keep the account token server-only, and route the existing refresh triggers through platform-specific query functions. New API reads raw account quota plus the upstream conversion unit; Sub2API accepts only an explicit USD wallet `balance` response. The existing `channels.balance` storage and refresh endpoints remain unchanged.

**Tech Stack:** Go 1.22, Gin, GORM v2, `httptest`, React 19, TypeScript, React Hook Form, Zod, Base UI, Tailwind CSS, i18next, Bun, Vitest.

---

## File Map

- Modify `model/channel.go`: persistent balance-query fields and response-only credential state.
- Modify `controller/channel.go`: hide the account token, preserve an empty token on edit, and audit sensitive changes.
- Modify `controller/channel_authz.go`: classify balance configuration fields by permission and read-only status.
- Modify `controller/channel_authz_test.go`: protect credential redaction and authorization behavior.
- Rewrite `controller/channel-billing.go`: platform dispatch, validation, upstream requests, response parsing, and refresh error logging.
- Create `controller/channel_balance_test.go`: account-balance query contract tests.
- Modify `web/default/src/features/channels/types.ts`: API/schema fields.
- Modify `web/default/src/features/channels/lib/channel-form.ts`: form fields and create/update transforms.
- Modify `web/default/src/features/channels/lib/channel-form.test.ts`: form round-trip and secret-preservation tests.
- Modify `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`: conditional balance configuration controls.
- Modify `web/default/scripts/add-missing-keys.mjs`: temporary six-locale translation additions; delete after applying.
- Update `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json` only through the translation script and `bun run i18n:sync`.
- Amend `docs/superpowers/specs/2026-07-17-channel-account-balance-query-design.md`: clarify that missing fields are validated during refresh rather than blocking channel save.

### Task 1: Clarify the Approved Spec

**Files:**
- Modify: `docs/superpowers/specs/2026-07-17-channel-account-balance-query-design.md`

- [ ] **Step 1: Replace the frontend-required validation statement**

Change the frontend test requirement from blocking required-field validation to conditional rendering and payload behavior:

```markdown
Frontend tests cover:

- Platform-dependent fields appear correctly.
- Balance configuration fields round-trip through create and edit payloads.
- Sub2API does not request duplicate credentials.
- Editing with an empty account access-token field preserves the existing secret.
```

- [ ] **Step 2: Verify the spec remains internally consistent**

Run:

```bash
rg -n "required-field|TBD|TODO|FIXME" docs/superpowers/specs/2026-07-17-channel-account-balance-query-design.md
git diff --check -- docs/superpowers/specs/2026-07-17-channel-account-balance-query-design.md
```

Expected: no stale `required-field` text, no placeholders, and no whitespace errors.

- [ ] **Step 3: Commit the clarification**

```bash
git add docs/superpowers/specs/2026-07-17-channel-account-balance-query-design.md
git commit -m "docs: clarify balance refresh validation"
```

### Task 2: Add Secure Channel Configuration Fields

**Files:**
- Modify: `model/channel.go`
- Modify: `controller/channel.go`
- Modify: `controller/channel_authz.go`
- Test: `controller/channel_authz_test.go`

- [ ] **Step 1: Write failing authorization and redaction tests**

Add cases that construct a channel with an account token and assert:

```go
channel := &model.Channel{
    BalancePlatform: "new_api",
    BalanceBaseURL:  "https://example.com",
    BalanceUserID:   1787,
    BalanceAuthKey:  "account-secret",
}
clearChannelInfo(channel)
assert.Empty(t, channel.BalanceAuthKey)
assert.True(t, channel.BalanceAuthKeyConfigured)
```

Extend field-classification tests so `balance_platform`, `balance_base_url`, `balance_user_id`, and `balance_auth_key` are classified sensitive, while `balance_auth_key_configured` is read-only.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run:

```bash
go test ./controller -run 'TestChannel.*(Classified|Sensitive|ReadOnly|Redact)' -count=1
```

Expected: FAIL because the new fields do not exist or are not classified.

- [ ] **Step 3: Add model fields**

Add to `model.Channel`:

```go
BalancePlatform          string `json:"balance_platform" gorm:"type:varchar(32);default:''"`
BalanceBaseURL           string `json:"balance_base_url" gorm:"type:varchar(512);default:''"`
BalanceUserID            int    `json:"balance_user_id" gorm:"default:0"`
BalanceAuthKey           string `json:"balance_auth_key,omitempty" gorm:"type:text"`
BalanceAuthKeyConfigured bool   `json:"balance_auth_key_configured" gorm:"-"`
```

The existing `AutoMigrate(&Channel{})` path supplies cross-database migration support.

- [ ] **Step 4: Redact and preserve the account token**

Update `clearChannelInfo`:

```go
channel.BalanceAuthKeyConfigured = strings.TrimSpace(channel.BalanceAuthKey) != ""
channel.BalanceAuthKey = ""
```

In `UpdateChannel`, after loading `originChannel`, preserve the secret when the request omits it or sends an empty value:

```go
if strings.TrimSpace(channel.BalanceAuthKey) == "" {
    channel.BalanceAuthKey = originChannel.BalanceAuthKey
}
```

Add the four editable fields to `channelSensitiveFields`, add precise comparisons in `channelHasSensitiveChanges`, add `balance_auth_key_configured` to `channelReadOnlyFields`, and clear that response-only flag in `clearChannelReadOnlyFields`.

Record only `balance_auth_key` in `changed_fields` when the stored secret changes; never record its value.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./controller -run 'TestChannel.*(Classified|Sensitive|ReadOnly|Redact)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit secure channel fields**

```bash
git add model/channel.go controller/channel.go controller/channel_authz.go controller/channel_authz_test.go
git commit -m "feat: add channel balance account settings"
```

### Task 3: Replace the Backend Balance Query Logic

**Files:**
- Modify: `controller/channel-billing.go`
- Create: `controller/channel_balance_test.go`

- [ ] **Step 1: Write failing New API query tests**

Use one `httptest.Server` with `/api/status` and `/api/user/self`. Assert the account request receives the exact headers and that raw quota converts correctly:

```go
channel := &model.Channel{
    BalancePlatform: "new_api",
    BalanceBaseURL:  server.URL + "/",
    BalanceUserID:   1787,
    BalanceAuthKey:  "account-token",
}

balance, err := queryNewAPIAccountBalance(channel)
require.NoError(t, err)
assert.InDelta(t, 1198.710618, balance, 1e-9)
```

The fixtures return `quota_per_unit: 500000` and `quota: 599355309`. Add table cases for missing URL, missing user ID, missing token, mismatched response user ID, unsuccessful response, zero conversion unit, negative quota, invalid JSON, and non-200 status.

- [ ] **Step 2: Write failing Sub2API query tests**

Use `httptest.Server` and assert the channel key is sent as a Bearer token:

```go
channel := &model.Channel{
    Key:             "sk-test",
    BalancePlatform: "sub2api",
    BalanceBaseURL:  server.URL,
}

balance, err := querySub2APIAccountBalance(channel)
require.NoError(t, err)
assert.Equal(t, 51.6, balance)
```

Add cases rejecting `quota_limited`, subscription-only responses, missing balance, non-USD units, negative/non-finite values, invalid JSON, and non-200 status.

- [ ] **Step 3: Run the new tests and confirm failure**

Run:

```bash
go test ./controller -run 'Test(QueryNewAPI|QuerySub2API|UpdateChannelAccountBalance)' -count=1
```

Expected: FAIL because the new functions are undefined.

- [ ] **Step 4: Implement shared request validation**

Replace the old provider-specific balance response types and subscription-minus-usage logic with:

```go
const (
    balancePlatformNewAPI  = "new_api"
    balancePlatformSub2API = "sub2api"
)

func normalizeBalanceBaseURL(rawURL string) (string, error) {
    normalized := strings.TrimRight(strings.TrimSpace(rawURL), "/")
    parsed, err := url.Parse(normalized)
    if err != nil || parsed.Scheme == "" || parsed.Host == "" {
        return "", errors.New("余额查询地址无效")
    }
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return "", errors.New("余额查询地址仅支持 HTTP 或 HTTPS")
    }
    return normalized, nil
}
```

Use `http.NewRequest`, `service.NewProxyHttpClient(channel.GetSetting().Proxy)`, `defer response.Body.Close()`, `io.LimitReader`, and `common.Unmarshal`. Do not add new direct JSON marshal/unmarshal calls.

- [ ] **Step 5: Implement New API account balance parsing**

Define response structs with only required fields and implement:

```go
func queryNewAPIAccountBalance(channel *model.Channel) (float64, error)
```

Request `{base}/api/status`, require `success`, and require finite `quota_per_unit > 0`. Request `{base}/api/user/self` with `Authorization`, `New-Api-User`, and `Accept` headers. Require `success`, matching `data.id`, and `data.quota >= 0`. Return `float64(quota) / quotaPerUnit` only if the result is finite and non-negative.

- [ ] **Step 6: Implement Sub2API wallet balance parsing**

Define:

```go
type sub2APIUsageResponse struct {
    Mode    string   `json:"mode"`
    Balance *float64 `json:"balance"`
    Unit    string   `json:"unit"`
}
```

Implement:

```go
func querySub2APIAccountBalance(channel *model.Channel) (float64, error)
```

Require `mode == "unrestricted"`, `balance != nil`, `unit` equal to `USD` case-insensitively, and a finite non-negative balance. Never read `remaining` or quota fields.

- [ ] **Step 7: Replace platform dispatch and preserve triggers**

Implement:

```go
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
```

Keep `UpdateChannelBalance`, `UpdateAllChannelsBalance`, and `AutomaticallyUpdateChannels` routes/signatures. In bulk refresh, log failures with channel ID and name, then continue. Preserve the existing multi-key rejection.

- [ ] **Step 8: Run backend tests**

Run:

```bash
go test ./controller -run 'Test(QueryNewAPI|QuerySub2API|UpdateChannelAccountBalance|Channel)' -count=1
go test ./model -run 'TestChannel' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit backend balance queries**

```bash
git add controller/channel-billing.go controller/channel_balance_test.go
git commit -m "feat: query upstream account balances"
```

### Task 4: Add Balance Configuration to the Channel Form

**Files:**
- Modify: `web/default/src/features/channels/types.ts`
- Modify: `web/default/src/features/channels/lib/channel-form.ts`
- Modify: `web/default/src/features/channels/lib/channel-form.test.ts`
- Modify: `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`

- [ ] **Step 1: Write failing form transformation tests**

Add tests asserting New API create payloads include:

```ts
expect(payload.channel).toMatchObject({
  balance_platform: 'new_api',
  balance_base_url: 'https://example.com',
  balance_user_id: 1787,
  balance_auth_key: 'account-token',
})
```

Add edit tests asserting a blank `balance_auth_key` is omitted, a replacement is included, trailing slashes are removed from `balance_base_url`, and Sub2API payloads clear `balance_user_id` while omitting `balance_auth_key`.

- [ ] **Step 2: Run the form tests and confirm failure**

Run:

```bash
cd web/default && bun run test -- src/features/channels/lib/channel-form.test.ts
```

Expected: FAIL because the balance fields are absent.

- [ ] **Step 3: Extend channel and form schemas**

Add to `channelSchema`:

```ts
balance_platform: z.enum(['', 'new_api', 'sub2api']).default(''),
balance_base_url: z.string().default(''),
balance_user_id: z.number().default(0),
balance_auth_key: z.string().optional(),
balance_auth_key_configured: z.boolean().default(false),
```

Add equivalent form fields and defaults. Do not block channel save when the configuration is incomplete; refresh-time backend validation is authoritative.

- [ ] **Step 4: Update create/edit transforms**

Parse the fields in `transformChannelToFormDefaults`. Normalize the balance URL with the existing `normalizeBaseUrl` helper. Include the account token on create only when non-empty and on edit only when replaced. When platform is `sub2api`, send `balance_user_id: 0` and omit the account token.

- [ ] **Step 5: Add conditional editor controls**

In the credentials section, render:

```tsx
<Select>
  <SelectItem value=''>{t('Not configured')}</SelectItem>
  <SelectItem value='new_api'>{t('New API')}</SelectItem>
  <SelectItem value='sub2api'>{t('Sub2API')}</SelectItem>
</Select>
```

Always show the balance query URL after a platform is selected. For New API, show a numeric user ID input and password-style account token input. On edit, use `balance_auth_key_configured` to display the description `Account access token is configured. Leave empty to keep it.` Sub2API displays a description that the existing channel API key will be reused.

Add all new fields to `SENSITIVE_FORM_FIELDS` so unsaved changes and permission behavior match existing credentials.

- [ ] **Step 6: Run frontend unit tests and type checking**

Run:

```bash
cd web/default
bun run test -- src/features/channels/lib/channel-form.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit frontend form behavior**

```bash
git add web/default/src/features/channels/types.ts web/default/src/features/channels/lib/channel-form.ts web/default/src/features/channels/lib/channel-form.test.ts web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx
git commit -m "feat: configure channel balance platforms"
```

### Task 5: Add Complete i18n Copy

**Files:**
- Modify temporarily: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/en.json`
- Modify through script: `web/default/src/i18n/locales/zh.json`
- Modify through script: `web/default/src/i18n/locales/fr.json`
- Modify through script: `web/default/src/i18n/locales/ja.json`
- Modify through script: `web/default/src/i18n/locales/ru.json`
- Modify through script: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Run the current i18n report**

```bash
cd web/default && bun run i18n:sync
```

Record the pre-existing report separately from new missing keys.

- [ ] **Step 2: Add translations through the required script**

Populate `newKeys` in `scripts/add-missing-keys.mjs` for every introduced UI key, including:

```text
Balance platform
Balance query URL
Not configured
New API user ID
New API account access token
Account access token is configured. Leave empty to keep it.
Sub2API reuses the channel API key for balance queries.
The platform base URL used for account balance queries.
```

Provide natural, compact translations for `en`, `zh`, `fr`, `ja`, `ru`, and `vi`.

- [ ] **Step 3: Apply and verify translations**

```bash
cd web/default
node scripts/add-missing-keys.mjs
bun run i18n:sync
rg -n 'Balance platform|Balance query URL|New API user ID' src/i18n/locales/*.json
```

Expected: every new key exists in all six locale files and the sync report has no new missing keys.

- [ ] **Step 4: Delete the temporary script**

Delete `web/default/scripts/add-missing-keys.mjs` after the locale updates are applied, as required by the i18n workflow.

- [ ] **Step 5: Commit translations**

```bash
git add web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "feat: translate channel balance settings"
```

### Task 6: Final Verification

**Files:**
- Verify all files changed by Tasks 1-5.

- [ ] **Step 1: Format changed code**

```bash
gofmt -w model/channel.go controller/channel.go controller/channel_authz.go controller/channel_authz_test.go controller/channel-billing.go controller/channel_balance_test.go
cd web/default && bunx oxfmt --write src/features/channels/types.ts src/features/channels/lib/channel-form.ts src/features/channels/lib/channel-form.test.ts src/features/channels/components/drawers/channel-mutate-drawer.tsx
```

Review formatter output so unrelated user files are not staged or committed.

- [ ] **Step 2: Run backend verification**

```bash
go test ./controller ./model
```

Expected: PASS.

- [ ] **Step 3: Run frontend verification**

```bash
cd web/default
bun run test -- src/features/channels/lib/channel-form.test.ts
bun run typecheck
bun run lint
bun run build
bun run i18n:sync
```

Expected: all commands complete successfully with no new i18n gaps.

- [ ] **Step 4: Inspect the final diff and credential safety**

```bash
git diff --check
git status --short
git diff --stat HEAD~5..HEAD
rg -n "balance_auth_key" controller model web/default/src/features/channels
```

Confirm no response path exposes the stored account token, no log prints credentials, no legacy subscription-minus-usage balance path remains, and unrelated pre-existing worktree changes are unstaged.

- [ ] **Step 5: Commit any verification-only fixes**

```bash
git add model/channel.go controller/channel.go controller/channel_authz.go controller/channel_authz_test.go controller/channel-billing.go controller/channel_balance_test.go web/default/src/features/channels/types.ts web/default/src/features/channels/lib/channel-form.ts web/default/src/features/channels/lib/channel-form.test.ts web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "test: verify channel account balance queries"
```

Skip this commit if verification required no code changes.

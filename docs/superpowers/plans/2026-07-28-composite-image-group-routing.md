# Composite Image Group Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add administrator-configured composite token groups whose administrator-defined public model routes image generation and image editing through ordered existing group/model targets with per-target retry counts and isolated billing.

**Architecture:** Add two independent database tables and an immutable policy snapshot. `TokenAuth` and `Distribute` detect composite groups before physical-group validation and channel selection; ordinary groups execute the existing branches unchanged. A dedicated image coordinator selects each physical group/internal model target, reuses the existing image relay and billing session, and advances only on configured retryable failures.

**Tech Stack:** Go 1.22, Gin, GORM v2, SQLite/MySQL/PostgreSQL, React 19, TypeScript, TanStack Query, React Hook Form, Zod, Base UI/Tailwind, Bun, testify.

---

## File Structure

- `model/composite_group.go`: additive entities and transactional CRUD.
- `service/composite_group.go`: validation, immutable snapshot, lookup, and refresh.
- `controller/composite_group.go`: administrator CRUD API.
- `controller/composite_image_relay.go`: ordered image routing, retries, billing, and fallback.
- `web/default/src/features/system-settings/models/composite-groups/`: administration page feature.
- Existing auth, distributor, relay info, pricing, group-list, router, migration, logging, and i18n files receive minimal gated additions.

## Task 1: Persistence and Atomic Policy Snapshot

**Files:**
- Create: `model/composite_group.go`
- Create: `model/composite_group_test.go`
- Modify: `model/main.go`
- Create: `service/composite_group.go`
- Create: `service/composite_group_test.go`

- [ ] **Step 1: Write failing model tests**

```go
func TestCreateCompositeGroupPreservesAdministratorPublicModel(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:composite-group?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&CompositeGroup{}, &CompositeGroupRoute{}, &Token{}))
    group := CompositeGroup{Name: "image_stable", PublicModel: "admin-image-model", Status: 1, UserSelectable: true}
    routes := []CompositeGroupRoute{{Operation: CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "gpt_image_web", InternalModel: "gpt-image-2-w", RetryCount: 2, Status: 1}}
    require.NoError(t, CreateCompositeGroup(db, &group, routes))
    got, err := GetCompositeGroupByID(db, group.Id)
    require.NoError(t, err)
    assert.Equal(t, "admin-image-model", got.PublicModel)
    require.Len(t, got.Routes, 1)
}
```

- [ ] **Step 2: Run the failing tests**

Run: `go test ./model -run 'Test.*CompositeGroup' -count=1`

Expected: FAIL because the entities and CRUD functions do not exist.

- [ ] **Step 3: Implement additive cross-database entities**

```go
const (
    CompositeOperationGeneration = "image_generation"
    CompositeOperationEdit = "image_edit"
)

type CompositeGroup struct {
    Id int `json:"id"`
    Name string `json:"name" gorm:"size:64;not null;uniqueIndex:uk_composite_group_name_deleted,priority:1"`
    PublicModel string `json:"public_model" gorm:"size:128;not null"`
    DisplayName string `json:"display_name" gorm:"size:128"`
    Description string `json:"description" gorm:"type:text"`
    Status int `json:"status" gorm:"index"`
    UserSelectable bool `json:"user_selectable"`
    PricingVisible bool `json:"pricing_visible"`
    GenerationEnabled bool `json:"generation_enabled"`
    EditEnabled bool `json:"edit_enabled"`
    CreatedTime int64 `json:"created_time" gorm:"bigint"`
    UpdatedTime int64 `json:"updated_time" gorm:"bigint"`
    DeletedAt gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_composite_group_name_deleted,priority:2"`
    Routes []CompositeGroupRoute `json:"routes" gorm:"-"`
}

type CompositeGroupRoute struct {
    Id int `json:"id"`
    CompositeGroupId int `json:"composite_group_id" gorm:"index;not null"`
    Operation string `json:"operation" gorm:"size:32;not null;index"`
    RouteOrder int `json:"route_order" gorm:"not null"`
    PhysicalGroup string `json:"physical_group" gorm:"size:64;not null"`
    InternalModel string `json:"internal_model" gorm:"size:128;not null"`
    RetryCount int `json:"retry_count"`
    RetryStatusCodes string `json:"retry_status_codes" gorm:"type:text"`
    Status int `json:"status" gorm:"index"`
    CreatedTime int64 `json:"created_time" gorm:"bigint"`
    UpdatedTime int64 `json:"updated_time" gorm:"bigint"`
    DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
```

Use GORM transactions and Go-side defaults. Reject deletion while a token references the group. Register both entities in normal and fast migrations.

- [ ] **Step 4: Write failing service tests**

Test empty public model, physical-group name conflict, missing routes, duplicate order, invalid retry count/status expression, lookup by token group, and refresh retaining the last valid snapshot.

- [ ] **Step 5: Implement validation and cache**

```go
func InitCompositeGroupCache() error
func RefreshCompositeGroupCache() error
func SyncCompositeGroupCache(frequency int)
func ResolveCompositeGroup(group string) (*CompositeGroupPolicy, bool)
func ListSelectableCompositeGroups(userGroup string) map[string]CompositeSelectableGroup
func ValidateCompositeGroup(group model.CompositeGroup, routes []model.CompositeGroupRoute) error
func ShouldRetryCompositeStatus(expression string, status int) bool
func HasCompositePolicyContext(c *gin.Context) bool
```

Use `atomic.Pointer[CompositePolicySnapshot]`; publish only a fully validated snapshot. Validate physical groups and abilities without adding composite models to channel abilities.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./model ./service -run 'Test.*Composite' -count=1`

Expected: PASS.

Commit: `git commit -m "feat: add composite group persistence" -- model/composite_group.go model/composite_group_test.go model/main.go service/composite_group.go service/composite_group_test.go`

## Task 2: Administrator CRUD API

**Files:**
- Create: `controller/composite_group.go`
- Create: `controller/composite_group_test.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write failing API tests**

Cover AdminAuth, list/detail, create with `public_model`, complete route replacement, validate without write, status enable validation, disable, and referenced-token deletion rejection.

- [ ] **Step 2: Run the failing tests**

Run: `go test ./controller -run 'Test.*CompositeGroup' -count=1`

Expected: FAIL because handlers are absent.

- [ ] **Step 3: Implement full-definition handlers**

```go
type compositeGroupRequest struct {
    model.CompositeGroup
    Routes []model.CompositeGroupRoute `json:"routes"`
}
```

Create/update writes the group and full route set in one transaction. Enablement validates again inside the transaction. After commit, refresh the local snapshot.

- [ ] **Step 4: Register admin routes**

```text
GET    /api/composite-groups
GET    /api/composite-groups/:id
POST   /api/composite-groups
PUT    /api/composite-groups/:id
PATCH  /api/composite-groups/:id/status
POST   /api/composite-groups/:id/validate
DELETE /api/composite-groups/:id
```

- [ ] **Step 5: Run tests and commit**

Run: `go test ./controller ./router -run 'Test.*CompositeGroup' -count=1`

Expected: PASS.

Commit only `controller/composite_group.go`, its tests, and the router hunk.

## Task 3: Token Group Discovery and Ordinary-Path Isolation

**Files:**
- Modify: `middleware/auth.go`
- Modify: `controller/group.go`
- Modify: `constant/context_key.go`
- Modify: `middleware/distributor.go`
- Create: `middleware/composite_group_test.go`

- [ ] **Step 1: Write failing isolation tests**

Prove:

```text
ordinary physical group -> existing GroupRatio and channel selection
enabled composite group -> no synthetic GroupRatio required
disabled composite group -> explicit rejection
composite group + wrong public model -> rejection before channel selection
composite group + configured public model -> skip initial physical selection
```

- [ ] **Step 2: Run the failing tests**

Run: `go test ./middleware ./controller -run 'Test.*CompositeGroup.*(Auth|Distribute|Groups)' -count=1`

Expected: FAIL.

- [ ] **Step 3: Add the gated TokenAuth branch**

```go
if policy, exists := service.ResolveCompositeGroup(tokenGroup); exists {
    if !policy.Enabled || !policy.UserSelectable {
        abortWithOpenAiMessage(c, http.StatusForbidden, "组合分组不可用")
        return
    }
    userGroup = tokenGroup
} else {
    // Keep the existing physical-group authorization block unchanged.
}
```

- [ ] **Step 4: Add group API union and distributor bypass**

Append selectable composite groups after physical groups in `GetUserGroups`. In `Distribute`, after model-limit validation but before affinity/selection, exact-match the administrator public model, validate generation/edit route, store composite policy context, set `original_model`, call `c.Next()`, and return without `SetupContextForSelectedChannel`.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./middleware ./controller -run 'Test.*CompositeGroup.*(Auth|Distribute|Groups)' -count=1`

Expected: PASS and ordinary-path assertions unchanged.

Commit only the composite group hunks and tests.

## Task 4: Billing Identity and Ordered Image Coordinator

**Files:**
- Modify: `relay/common/relay_info.go`
- Modify: `relay/helper/price.go`
- Modify: `relay/helper/price_test.go`
- Create: `controller/composite_image_relay.go`
- Create: `controller/composite_image_relay_test.go`
- Modify: `controller/relay.go`
- Modify: `service/log_info_generate.go`

- [ ] **Step 1: Write failing billing-identity tests**

```go
func TestModelPriceHelperUsesBillingModelName(t *testing.T) {
    info := &relaycommon.RelayInfo{OriginModelName: "admin-image-model", BillingModelName: "gpt-image-2-w"}
    // Configure price only for gpt-image-2-w and assert fixed-price billing is selected.
}
```

- [ ] **Step 2: Add billing identities**

```go
type RelayInfo struct {
    // existing fields
    BillingModelName string
    CompositeGroupName string
    CompositeRouteOrder int
    CompositeOperation string
}

func (info *RelayInfo) EffectiveBillingModelName() string {
    if info.BillingModelName != "" { return info.BillingModelName }
    return info.OriginModelName
}
```

Use the effective billing name for model price/ratio/expression lookup while retaining the public model for user-visible logs and subscription eligibility.

- [ ] **Step 3: Write failing coordinator tests**

Cover generation/edit policy selection, first-target success, retry count zero, same-target retries, retryable 429/5xx/transport fallback, non-retryable 4xx stop, multipart body replay, response-commit stop, fixed-to-token repricing, reserve failure, and all-target refund.

- [ ] **Step 4: Add the early relay gate**

```go
if relayFormat == types.RelayFormatOpenAIImage && service.HasCompositePolicyContext(c) {
    relayCompositeImage(c, relayFormat)
    return
}
```

Do not restructure the ordinary relay body.

- [ ] **Step 5: Implement the coordinator**

Parse/validate once and estimate tokens once. For each configured target, set the physical group and billing model, calculate the target price snapshot, create or reserve the billing session, then attempt from zero through `RetryCount` inclusive. Select channels with the target physical group/internal model and existing priority/weight logic. Rewind `BodyStorage` before every attempt. Advance only on retryable errors before response commitment.

- [ ] **Step 6: Add admin-only route evidence**

Put composite group, operation, route order, billing model, physical group, and failed attempt history under `other.admin_info`.

- [ ] **Step 7: Run tests and commit**

Run: `go test ./controller ./service ./relay/helper -run 'TestComposite|TestModelPriceHelperUsesBillingModelName' -count=1`

Expected: PASS.

Commit only the coordinator, billing identity, price helper, log metadata, and related tests.

## Task 5: Administration Page

**Files:**
- Create: `web/default/src/features/system-settings/models/composite-groups/types.ts`
- Create: `web/default/src/features/system-settings/models/composite-groups/api.ts`
- Create: `web/default/src/features/system-settings/models/composite-groups/schema.ts`
- Create: `web/default/src/features/system-settings/models/composite-groups/schema.test.ts`
- Create: `web/default/src/features/system-settings/models/composite-groups/composite-groups-section.tsx`
- Create: `web/default/src/features/system-settings/models/composite-groups/composite-group-drawer.tsx`
- Create: `web/default/src/features/system-settings/models/composite-groups/route-target-editor.tsx`
- Modify: `web/default/src/features/system-settings/models/section-registry.tsx`

- [ ] **Step 1: Write failing schema tests**

Test required group/public model, enabled operation routes, unique order, bounded retry count, and full request serialization.

- [ ] **Step 2: Implement API, types, schema, page, and drawer**

Use React Query for CRUD, React Hook Form + Zod for the full definition, separate Generation/Edit route lists, sortable target rows, physical group/internal model selectors, retry count, status codes, and read-only billing mode.

- [ ] **Step 3: Register the route section**

```tsx
{
  id: 'composite-groups',
  titleKey: 'Composite Groups',
  build: () => <CompositeGroupsSection />,
}
```

- [ ] **Step 4: Run frontend checks and commit**

Run from `web/default`:

```text
bun test src/features/system-settings/models/composite-groups/schema.test.ts
bun run typecheck
```

Expected: PASS.

Commit the new feature directory and section-registry hunk.

## Task 6: Internationalization

**Files:**
- Modify: `web/default/src/i18n/static-keys.ts`
- Modify: every supported locale JSON under `web/default/src/i18n/locales/`

- [ ] **Step 1: Register and translate all new keys**

Include Composite Groups, Public request model, Generation routes, Edit routes, Physical group, Internal model, Retry count, Retry status codes, Validate configuration, and every validation/toast message.

- [ ] **Step 2: Run i18n synchronization and checks**

Run from `web/default`:

```text
bun run i18n:sync
bun run i18n:check
```

Expected: no missing composite-group keys.

- [ ] **Step 3: Commit translations**

Commit only i18n files changed for this feature.

## Task 7: Regression Verification and Deployment Boundary

**Files:**
- Test-only fixes if a real contract gap is found.

- [ ] **Step 1: Run affected backend tests**

Run: `go test ./model ./service ./middleware ./controller ./router ./relay/... -count=1`

Expected: PASS.

- [ ] **Step 2: Run frontend checks**

Run from `web/default`: `bun test && bun run typecheck && bun run lint && bun run build`

Expected: PASS.

- [ ] **Step 3: Audit the four isolation contracts**

```text
ordinary group + gpt-image-2-w -> existing selector and fixed billing
ordinary group + gpt-image-2   -> existing selector and token billing
composite group + administrator public model -> configured priority/retries/fallback
composite group + wrong model -> rejected before channel selection
```

- [ ] **Step 4: Review the final diff**

Run: `git status --short`, `git diff --stat`, and `git diff --check`.

Expected: composite-group files plus pre-existing user work only; no whitespace errors.

- [ ] **Step 5: Preserve deployment boundary**

Do not deploy relay workers without explicit user command. The feature remains unused until all relay-serving instances have the new code and an administrator creates/enables a composite group.

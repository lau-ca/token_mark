# Seedance Task 表达式计费与视频 URL 隐藏设计

## 决策

`videos-fast`、`videos-mini`、`videos-standard` 的业务售价由现有 `tiered_expr` 统一管理。新增通用 `per_request(amount)` 表达式函数，并通过 Task 专用计费入口接入异步视频任务。

不继续在 Sora adaptor 中硬编码价格倍率，不新增 `task_expr`，不把本次改造扩展为全站退款账本重构。

## 目标

- 同一个模型根据 `resolution` 和 `duration` 支持按次、按秒混合计费。
- `Seedance2.0` 分组倍率为 `1` 时，最终业务价格与表达式中的数字完全一致。
- 参数校验、预扣、提交失败退款、异步失败退款和日志继续复用现有 Task 生命周期。
- 渠道重试期间冻结首次计费结果，不能因配置更新或渠道切换改变本次请求价格。
- 旧 Task、Midjourney、Sora 和其他渠道保持原有计费路径。
- 删除此前开发的 Seedance 硬编码价格代码、对应失效测试和过时文档，保持 Git diff 聚焦、可审查。
- 保留已经完成的渠道级视频 URL 替换功能及其默认关闭、按渠道隔离边界。

## 非目标

- 不新增数据库表或字段迁移。
- 不新增第四种 billing mode。
- 不给可视化表达式编辑器增加视频价格表格。
- 不在本次实现全站 exactly-once 退款账本。
- 不为三个 Seedance 模型开放 remix；现有 remix 的上游 ID 和计费参数继承需另行修复。
- 不重构与本功能无关的 Task、MJ、表达式或前端设置代码。

## 当前问题

### Task 不执行表达式

文本计费入口会优先检查 `tiered_expr`，但 Task 使用的 `ModelPriceHelperPerCall` 只读取 `ModelPrice` / `ModelRatio`。直接在页面配置表达式不会影响 `/v1/videos` 的实际扣费。

`ModelPriceHelperPerCall` 同时被 Midjourney 使用，不能直接改成 Task 表达式逻辑，否则可能改变 MJ 的额度字段和扣费结果。

### 现有 Seedance 硬编码价格已不符合业务售价

当前 `seedanceBillingProfiles` 依据旧价格生成 `OtherRatios`。若只把基础 ModelPrice 改成新售价，会得到错误结果，例如：

- `videos-fast` 720p：`4 × 1.75 = 7`，目标为 `6`。
- `videos-mini` 720p：`2.5 × 5/3 = 4.1667`，目标为 `3.5`。
- `videos-standard` 1080p：约 `0.9429/秒`，目标为 `0.9/秒`。
- `videos-standard` 4k：约 `1.8857/秒`，目标为 `2/秒`。

价格不应继续存在于协议适配器中。

### 配置保存不安全

当前 `billing_mode` 与 `billing_expr` 分开保存。Default 前端顺序保存且先保存 mode，Classic 前端并发保存；后端已有 `SmokeTestExpr` 但没有调用方。这会产生模式已启用、表达式尚未生效的窗口。

### 当前退款不是财务级绝对保证

正常提交失败会通过 `BillingSession.Refund` 退还预扣；正常异步失败会按 `task.Quota` 退款。但现有全局 Task 代码仍有终态 CAS 后进程崩溃窗口，以及上游任务 ID 为空、渠道读取失败等直接标记失败却不执行退款的分支。

本次必须保证 Seedance 表达式任务不削弱现有正常退款路径，并通过回归测试保护；全站 exactly-once 退款需要独立的持久化账本和幂等状态机，不与本次定制渠道改造混合。

## 表达式设计

### `per_request(amount)`

新增 v1 内置函数：

```text
per_request(amount) = amount * 1_000_000
```

现有 v1 quota 转换保持不变：

```text
quota = expr_output / 1_000_000 * QuotaPerUnit * groupRatio
```

因此 `per_request(2.5)` 表示真实的每请求价格 `2.5`。`1_000_000` 只存在于表达式引擎内部，管理员、价格页面和日志不需要理解内部换算。

`per_request()` 同时作为 Task 表达式的显式 opt-in 标记：

- `tiered_expr` 且表达式使用 `per_request()`：进入 Task 表达式计费。
- 不使用 `per_request()`：继续原 Task/MJ 路径，不改变历史模型行为。

### 三个模型表达式

`videos-mini`：

```text
param("resolution") == "480p"
  ? tier("480p", per_request(2.5))
  : param("resolution") == "720p"
    ? tier("720p", per_request(3.5))
    : tier("invalid", -1)
```

`videos-fast`：

```text
param("resolution") == "480p"
  ? tier("480p", per_request(4))
  : param("resolution") == "720p"
    ? tier("720p", per_request(6))
    : tier("invalid", -1)
```

`videos-standard`：

```text
param("resolution") == "480p"
  ? tier("480p", per_request(5.5))
  : param("resolution") == "720p"
    ? tier("720p", per_request(8))
    : param("resolution") == "1080p"
      ? tier("1080p", per_request(0.9) * param("duration"))
      : param("resolution") == "4k"
        ? tier("4k", per_request(2) * param("duration"))
        : tier("invalid", -1)
```

请求校验会先拒绝未知规格；表达式中的 `invalid` 分支作为第二层防御，运行时负值必须 fail-closed，不能转成负扣费或免费请求。

## Task 计费数据流

### 1. 参数规范化

Sora adaptor 完成参数校验后，从上下文读取规范化的 `TaskSubmitReq`，只构造计费需要的输入：

```json
{
  "model": "videos-standard",
  "resolution": "4k",
  "duration": 15
}
```

这样 JSON、multipart、字符串 duration、兼容 seconds 和 `4K` 大小写输入都会得到相同计费结果。完整 prompt、参考图片、视频、音频 URL 不进入持久化计费上下文。

### 2. Task 专用价格入口

新增 Task 专用 helper：

1. 检查模型 billing mode 和表达式。
2. 仅在表达式使用 `per_request()` 时运行 Task 表达式。
3. 以 `p=0`、`c=0` 和规范化请求参数执行表达式。
4. 拒绝负数、NaN、正负无穷和运行错误。
5. 使用 `common.QuotaRoundChecked` 计算额度并记录饱和审计。
6. 将最终额度写入 Task 使用的 `PriceData.Quota`。

原 `ModelPriceHelperPerCall` 保持不变，MJ 和未启用 Task 表达式的模型继续使用原逻辑。

### 3. 首次计算冻结

首次尝试生成 BillingSnapshot，包含：

- billing mode；
- 模型名；
- 表达式 hash；
- 表达式文本或可审计的冻结副本；
- group ratio；
- 最终预扣额度；
- matched tier。

后续渠道重试必须复用该 snapshot，不能重新读取页面上的最新表达式。冻结判断基于 snapshot 是否存在，不能依赖 BillingSession 是否存在，因为免费分组可能没有 BillingSession。

### 4. 防止重复计费

Task 表达式生效时必须跳过：

- adaptor `EstimateBilling`；
- `OtherRatios` 乘法；
- adaptor `AdjustBillingOnSubmit`；
- 轮询完成时的 token/adaptor 差额重算。

任务写入时设置现有 `TaskBillingContext.PerCallBilling=true`。这里的含义是“最终费用已在提交时确定”，包括按请求 duration 计算出的按秒价格。

### 5. 任务持久化和日志

Task 私有计费上下文只保存必要审计字段：

- billing mode；
- expression hash；
- matched tier；
- origin model；
- group ratio；
- 最终 quota；
- 规范化的 `resolution` 和 `duration`。

不得保存完整请求体、prompt、参考媒体 URL 或请求头。

提交消费日志和退款日志显示：

- `billing_mode=tiered_expr`；
- matched tier；
- expression hash 或安全表达式副本；
- group ratio；
- 最终 quota；
- quota saturation 审计信息。

## 退款边界

### 本次必须保证

- 参数校验失败发生在预扣前。
- 上游提交前后的同步失败继续触发 `BillingSession.Refund`。
- 上游接受任务后，任务记录保存本次最终 `task.Quota`。
- 正常异步失败继续调用 `RefundTaskQuota`，按 `task.Quota` 原额退还钱包/订阅和 Token 额度。
- 终态 CAS 仍阻止多个轮询节点重复执行正常退款。
- 表达式任务不得在完成或失败时重新读取新价格。

### 独立后续项目

以下财务级问题不在本次范围：

- CAS 成功后进程崩溃造成的退款窗口；
- 钱包退款的持久化幂等键；
- 退款失败的 outbox/重试队列；
- 全站历史任务的退款状态迁移。

不能在本次交付中宣称“任何进程故障下 exactly-once 退款”。

## Remix

三个 Seedance 模型暂时拒绝 `/v1/videos/{id}/remix`：

- remix 请求没有可靠的规范化 `resolution` / `duration`；
- 当前从原任务复制的倍率会被新 PriceData 覆盖；
- Sora remix 上游路径仍可能使用公开 task ID，而不是 upstream task ID。

拒绝逻辑只匹配三个 Seedance 模型，旧 Sora 模型保持现状。

## 配置保存与校验

### 后端原子保存

模型定价页面使用专用批量保存入口，内部复用 `model.UpdateOptionsBulk`，在同一个数据库事务中保存相关 pricing options，至少保证 `billing_setting.billing_mode` 与 `billing_setting.billing_expr` 原子提交。

事务提交前：

1. 解析 mode 和 expression map。
2. 编译所有新增或修改的表达式。
3. 对 `per_request()` 表达式执行有限值和非负校验。
4. 对三个 Seedance 模型执行 480p、720p、1080p、4k 样例，以及 duration 4、15 边界测试。
5. 任一表达式失败则整个保存失败，内存和数据库均不更新。

### 前端安全行为

- Default 和 Classic 使用同一个原子保存 API。
- HTTP 200 但 `success:false` 必须作为保存失败处理，不能更新本地“已保存”快照。
- `per_request()` / `param()` 请求表达式保持 raw-only。
- 无法反解析时禁止从 raw 切换到 visual，不能生成零价默认表达式覆盖原配置。
- Token 估算器检测到请求表达式时隐藏，提示以服务端校验为准，避免 `param()` 被当作 null 后显示错误价格。

## 前端价格和日志展示

Default 和 Classic 的表达式解析器改用括号平衡扫描，不能继续用遇到第一个 `)` 就结束的正则。

解析器支持：

- `per_request(2.5)` → `2.5/次`；
- `per_request(0.9) * param("duration")` → `0.9/秒`；
- 历史纯数字表达式继续兼容。

模型价格详情和日志按匹配 tier 展示“按次”或“每秒”；无法结构化解析时显示原始表达式，不伪造零价或 Token 价格。

## 删除旧错误实现

实施时必须以前向删除方式清理：

- 删除 `relay/channel/task/sora/constants.go` 中的 `seedanceBillingRule` 和 `seedanceBillingProfiles`。
- 删除 `relay/channel/task/sora/adaptor.go` 中基于该 profile 返回 Seedance `OtherRatios` 的分支。
- 删除或重写只验证旧硬编码倍率的 adaptor 测试。
- 删除旧固定价格 `videos-fast=2`、`videos-mini=1.5`、`videos-standard=3.5` 的生产配置；表达式成为唯一价格真相。
- 删除过时注释、过时集成方案和不再使用的 helper。

必须保留：

- 三个模型常量和 Sora ModelList 注册；
- Seedance 时长、分辨率及输入参数校验；
- JSON/multipart 请求规范化和上游转发；
- 旧 Sora 模型的 seconds/size 计费；
- 视频 URL 隐藏和 `/content` 代理实现。

严禁使用任何 Git rollback 操作。所有清理都通过明确的前向编辑完成。

## 渠道 URL 隐藏

现有渠道开关 `replace_video_urls_with_proxy` 设计保持不变：

- 默认关闭；
- 仅 OpenAI/Sora 渠道可启用；
- 开启后普通用户只收到平台鉴权 `/content` 地址；
- 上游真实 URL 仅保留在内部任务数据中；
- `/content` 保持任务所有权检查、Range/206、416 和安全响应头策略；
- 管理员任务视图保留排障数据；
- 不支持的渠道不解析、不修改该设置。

表达式计费与 URL 隐藏是两个独立开关/数据流，不互相耦合。

## 兼容性隔离

- Task 表达式只在 `tiered_expr + per_request()` 时启用。
- Seedance 参数和 remix 限制只匹配三个精确模型名。
- MJ 继续调用原 `ModelPriceHelperPerCall`。
- 旧 Sora 模型继续使用原 adaptor 计费。
- URL 替换继续由渠道开关控制。
- 表达式按模型名全局生效；上线前已确认三个模型当前只存在于渠道 `#114`。
- 若未来其他渠道复用相同模型名，会共享同一模型表达式；需要渠道级售价时应另行设计 channel+model override。

## 上线顺序

渠道 `#114` 在整个部署和配置阶段保持禁用。

1. 完成代码和自动化测试。
2. 手动前向更新 master。
3. 手动前向更新所有仍在提供 `/v1/videos` 的 worker；已删除的 `159.89.150.206` 忽略。
4. 逐节点确认版本、健康状态和配置同步。
5. 使用原子保存接口写入三个表达式并删除旧固定价格。
6. 将渠道 `#114` 分组从 `default` 改为 `Seedance2.0`，保持禁用。
7. 对所有规格并发测试，保存完整输入、上游输出、平台输出、状态轮询和计费日志。
8. 核对八个价格点、URL 隐藏和失败退款后再启用渠道。

旧 worker 会静默走旧 ModelPrice/硬编码价格，而不是可靠报错，因此禁止在所有 worker 更新完成前保存并启用表达式配置。

## 测试矩阵

### 表达式引擎

- `per_request()` 编译和运行。
- 与 token 表达式混用时不改变历史结果。
- 负数、NaN、Inf 和溢出 fail-closed/审计。
- UsedVars 能识别 `per_request` opt-in。

### Task 计费

- Mini：480p=2.5，720p=3.5。
- Fast：480p=4，720p=6。
- Standard：480p=5.5，720p=8。
- Standard 1080p：4 秒=3.6，15 秒=13.5。
- Standard 4k：4 秒=8，15 秒=30。
- group ratio=1 时额度与业务价格精确对应。
- JSON、multipart、duration 数字/字符串、seconds 兼容输入结果一致。
- 4K 规范化为 4k。
- 非法时长、分辨率在预扣前返回 400。
- 渠道重试只计算一次价格。
- 表达式任务跳过 adaptor 三个调价阶段。
- 免费分组同样冻结 snapshot。

### 退款和任务

- 提交失败退回钱包、订阅和 Token 额度。
- 正常异步失败按 `task.Quota` 原额退款。
- CAS 失败的轮询节点不重复退款。
- 表达式任务完成后不做差额重算。
- 三个 Seedance 模型 remix 返回明确的 4xx。
- 旧 Sora/MJ/其他 Task 行为保持不变。

### 配置和前端

- mode + expr 原子保存；任一表达式错误时全部不落库。
- Default/Classic 保存失败不会标记成功。
- raw→visual 不覆盖请求表达式。
- `2.5/次`、`0.9/秒` 等展示正确。
- 历史表达式展示保持兼容。

### URL 隐藏

- 开关关闭时响应兼容。
- 开启时普通用户不见上游 URL、签名参数和 upstream task ID。
- `/content` Range/206/416 和敏感响应头处理不回归。
- 不支持渠道完全不受影响。

## Git 变更要求

- 只修改实现本设计所必需的文件。
- 不做无关重构、目录调整或格式化整仓代码。
- 不改动用户已有的无关 dirty changes。
- 旧错误代码删除后不得保留备用分支、注释块或重复实现。
- 测试替换旧合同，不同时保留互相矛盾的旧/新价格测试。
- 每个阶段检查 `git diff --check`、变更文件列表和敏感信息。
- 代码提交与文档、测试产物、生产测试输入输出分开管理；测试密钥不得进入 Git。

## 验收标准

- 页面中的表达式是三个模型业务售价的唯一真相。
- 八个规格价格全部精确，分组倍率 1 时无隐藏倍率。
- 旧硬编码价格实现和失效测试全部删除。
- 正常失败退款不回归，且不夸大为全故障 exactly-once 保证。
- master 与全部存活 worker 使用同一版本后才允许启用渠道。
- 渠道 `#114` 为 Sora、`Seedance2.0` 分组、URL 替换开启且最终启用。
- 其他渠道、MJ、旧 Sora 和非表达式 Task 的行为与改动前一致。
- Git diff 聚焦、无无关代码、无密钥、无过时实现。

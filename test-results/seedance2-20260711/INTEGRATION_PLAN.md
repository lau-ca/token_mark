# New API 视频模型最小集成方案（退款链路复核版）

## 结论

现有异步任务计费框架已经具备以下能力，不应为 Seedance 2.0 重写：

- 请求提交前按预计价格全额预扣。
- 上游提交失败时，通过 `BillingSession.Refund` 退回钱包或订阅额度及 Token 额度。
- 上游任务最终失败或超时时，通过任务状态 CAS 控制，只允许一个轮询进程执行全额退款。
- 固定 `ModelPrice` 任务会保存计费快照，并在任务成功时跳过重复结算。

本次需要开发的核心只有两项：

1. 三个公开模型的参数归一化和校验。
2. 按模型、分辨率、时长生成现有 `OtherRatios`。

同时，源码审计发现两条独立的通用任务可靠性缺口。它们不要求建设第二套退款系统，但应在生产启用前用现有 CAS 和退款函数补齐。

## 一、现有计费与退款链路

### 1. 提交阶段

`relay/relay_task.go` 的任务提交顺序为：

1. 解析并验证请求。
2. 应用模型映射。
3. 读取 `ModelPrice`。
4. 调用适配器 `EstimateBilling` 获取 `OtherRatios`。
5. 计算最终预计额度。
6. 通过 `PreConsumeBilling` 全额预扣。
7. 提交上游任务。

如果构建请求、调用上游或解析创建响应失败，`controller/relay.go` 的 defer 会调用 `BillingSession.Refund`。同一个请求的渠道重试共享一个 BillingSession，因此不会每重试一次就重复预扣或退款。

### 2. 任务终态

轮询取得成功或失败终态后，`service/task_polling.go` 先调用 `Task.UpdateWithStatus` 执行 CAS：

- CAS 获胜且任务成功：执行既有成功结算。
- CAS 获胜且任务失败：调用 `RefundTaskQuota` 全额退款。
- CAS 失败：说明其他节点已经处理终态，本节点不再结算或退款。

超时任务使用相同的 CAS 后退款方式。钱包、订阅和 Token 额度的实际调整统一位于 `service/task_billing.go`，不需要模型专用退款代码。

### 3. 在线验证

两次 `videos-standard / 4k / 4s` 均预扣 `2,400,000 quota = ¥4.80`，任务在 89% 失败后均完整退回 `2,400,000 quota`。最终净扣费为 `10,200,000 quota = ¥20.40`，正好等于七个成功档位价格之和。

证据见 `billing_summary.json` 和两个 4k 用例目录中的 `usage_before.json`、`usage_after.json`、`final_response.json`。

## 二、方案选择

### 推荐：三个公开模型 + 适配器价格规格

继续对外暴露：

- `videos-fast`
- `videos-mini`
- `videos-standard`

客户端通过 `resolution` 选择规格。适配器在预扣前同时完成参数校验和价格倍率计算。

优点：

- 保持用户示例中的三模型 API。
- 能在服务端校验模型、分辨率和时长的一致性。
- 不允许用低价模型名请求高价分辨率。
- 直接复用 `ModelPrice`、`OtherRatios`、预扣、结算和退款。

### 备选：八个分辨率后缀 SKU

OpenAI/Sora 渠道可以让客户端直接调用 `videos-standard-1080p`，再通过静态 ModelMapping 让上游收到 `videos-standard`。六个按次 SKU 可加入 `TASK_PRICE_PATCH`，1080p/4k SKU 使用按秒基础价。

该方案只适合受信内部调用，不适合作为公开 API：配置层无法阻止客户端调用 `videos-standard-480p` 却在请求体中发送 `resolution: "4k"`，存在确定的少扣费风险。因此不作为正式方案。

## 三、模型与渠道配置

先在管理端确认真实渠道类型。在线 `/v1/models` 把这些模型标记为 `supported_endpoint_types: ["openai"]`，创建接口也直接接受顶层兼容字段，因此优先按 OpenAI/Sora task adaptor 集成。

渠道 `Models` 加入三个公开模型。ModelMapping 只负责把公开模型映射到实际上游模型，不承担计费：

```json
{
  "videos-standard": "实际标准版上游模型",
  "videos-fast": "实际快速版上游模型",
  "videos-mini": "实际轻量版上游模型"
}
```

如果最终确认使用火山方舟 `DoubaoVideo` 渠道，应把同一参数和价格规格落在 Doubao adaptor，并额外把顶层参考素材转换为官方 `content[]`。不要同时维护两套生效逻辑。

## 四、公开请求契约

继续接受：

```json
{
  "model": "videos-standard",
  "prompt": "...",
  "duration": 4,
  "ratio": "16:9",
  "resolution": "1080p",
  "referenceImages": ["https://..."],
  "referenceVideos": ["https://..."],
  "referenceAudios": ["https://..."]
}
```

在预扣前统一完成：

- `duration` 必须是整数 4～15。字符串整数可按现有兼容行为保留；小数、布尔值、超大整数和无法解析的字符串必须返回 400。
- Fast、Mini 仅允许 480p、720p。
- Standard 允许 480p、720p、1080p、4k。
- `ratio` 按实际上游支持的枚举校验。
- 图片 1～9 张；视频和音频各 0～3 个。
- 音频不能作为唯一参考素材。
- 多模态参考数组不能与 `first_frame`、`last_frame` 模式混用。
- 顶层规范字段优先于 `metadata`，并禁止 `metadata` 绕过时长和分辨率限制。

火山官方原生格式使用 `content[]`，而不是三个顶层 `reference*` 数组。若走 Sora 兼容渠道，保持原始 JSON 透传；若走 DoubaoVideo，才在 adaptor 中转换为官方格式。

## 五、计费规格

使用三个模型的 480p 价格作为 `ModelPrice` 基准。设站点价格配置的货币换算系数为 `R`：

- `videos-fast = 2 / R`
- `videos-mini = 1.5 / R`
- `videos-standard = 3.5 / R`

本次在线站点 `quota_display_type=CNY`、`usd_exchange_rate=1`，因此配置数值分别为 2、1.5、3.5。

适配器返回：

| 模型 | 分辨率 | `OtherRatios` |
|---|---|---|
| videos-fast | 480p | `resolution = 1` |
| videos-fast | 720p | `resolution = 1.75` |
| videos-mini | 480p | `resolution = 1` |
| videos-mini | 720p | `resolution = 5/3` |
| videos-standard | 480p | `resolution = 1` |
| videos-standard | 720p | `resolution = 10/7` |
| videos-standard | 1080p | `resolution = 6/35`，`seconds = duration` |
| videos-standard | 4k | `resolution = 12/35`，`seconds = duration` |

这样得到：

- Fast 480p：¥2.00/次。
- Fast 720p：¥3.50/次。
- Mini 480p：¥1.50/次。
- Mini 720p：¥2.50/次。
- Standard 480p：¥3.50/次。
- Standard 720p：¥5.00/次。
- Standard 1080p：¥0.60/秒。
- Standard 4k：¥1.20/秒。

不要把三个基础模型加入 `TASK_PRICE_PATCH`，否则同一模型下所有分辨率都会跳过 `OtherRatios`。

## 六、最小代码范围

### 模型计价适配包

- `relay/common/relay_info.go`
  - 增加 `resolution`、`ratio` 和三个 `reference*` 字段。
  - 对显式但无效的 `duration` 返回解析错误，不能静默回落到默认 4 秒。
- `relay/common/relay_utils.go`
  - 严格处理无法解析的 `seconds`。
  - 保留通用最大时长防溢出校验。
- `relay/channel/task/sora/constants.go`
  - 定义三个公开模型、允许的分辨率和价格倍率。
- `relay/channel/task/sora/adaptor.go`
  - 对三个模型执行 Seedance 专用参数校验。
  - `EstimateBilling` 根据模型、分辨率和时长返回上述倍率。
  - 其他 Sora 模型继续使用现有 seconds/size 逻辑。
- `relay/channel/task/sora/adaptor_test.go`
  - 表驱动覆盖八个价格点和所有非法组合。
- `relay/common/relay_utils_test.go`
  - 覆盖超大整数、小数、非法字符串和边界时长。

### 明确不修改

- `types/price_data.go`
- `service/billing.go`
- `service/billing_session.go`
- `service/funding_source.go`
- `service/task_billing.go` 的正常退款实现
- `model/task.go` 的 CAS 实现
- 用户、Token、订阅额度模型
- 系统任务调度器和租约逻辑

## 七、生产前可靠性补丁

这部分与模型价格适配分开提交，但仍只复用现有任务计费能力。

### 1. 渠道读取失败必须退款

`service/task_polling.go` 当前在 `CacheGetChannel` 失败时调用无 CAS 的 `TaskBulkUpdateByID` 批量标记失败，没有退款。这违反了 `TaskBulkUpdateByID` 自身声明的计费生命周期约束。

修复方式：

1. 逐个读取内存中的任务快照。
2. 设置失败原因、失败状态、100% 进度和完成时间。
3. 调用 `UpdateWithStatus(oldStatus)`。
4. 仅 CAS 获胜者调用 `RefundTaskQuota`。

### 2. 任务持久化失败不能留下扣费

`controller/relay.go` 当前在上游创建成功后先结算和记录消费日志，再执行 `task.Insert()`。插入失败时，用户已扣费但没有可轮询、可超时或可退款的任务记录。

需要单独设计并测试任务持久化、结算、响应输出的顺序，最低要求为：

- 插入失败不得留下净扣费。
- `SettleBilling` 失败不得继续按成功记录消费日志。
- 不得向客户端同时写入成功响应和第二个错误响应。
- 不得直接重复调用非幂等的钱包退款。

该调整涉及所有异步任务，实施前应对各 task adaptor 的 `DoResponse` 写响应行为做一次统一检查，不在 Seedance 价格函数内局部处理。

## 八、测试与验收

### 计价适配测试

- 八个规格计算后的 quota 与目标价格精确一致。
- 2 秒、16 秒、非法时长在预扣前拒绝。
- Fast/Mini 的 1080p、4k 在预扣前拒绝。
- 未知 resolution 在预扣前拒绝。
- 显式无效时长不能按默认 4 秒计费后继续透传。
- 三类参考素材正确保留或转换。
- 其他 Sora 模型原有 seconds/size 计费不回归。

### 退款与可靠性测试

现有钱包、订阅、Token、CAS 和 PerCallBilling 测试继续保留，不重复建设同义测试。新增：

- 渠道读取失败时，CAS 获胜任务只退款一次。
- 并发状态变化导致 CAS 失败时不退款。
- 任务插入失败不产生净扣费和成功消费日志。
- 结算失败不把任务当作正常成功提交。

### 建议命令

```bash
go test ./relay/common ./relay/channel/task/sora ./service ./model
```

生产验证时再使用临时 Key 并发提交八个有效档位。六个按次档位使用 15 秒；1080p、4k 使用官方最短 4 秒。预期总预扣 ¥25.20，失败任务必须全额退回。

## 九、实施顺序

1. 管理端确认真实 channel type、上游模型 ID 和 ModelMapping。
2. 完成模型计价适配包及单元测试。
3. 完成两项生产前可靠性补丁及回归测试。
4. 配置三个 `ModelPrice`，不配置基础模型的 `TASK_PRICE_PATCH`。
5. 在测试环境执行八档并发验证。
6. 核对消费日志中的模型、分组倍率、resolution、seconds、task ID、退款原因。
7. 单独处理 `/content` 的 Range/206 和 HTTPS 重定向问题；不与计价补丁混在同一提交。


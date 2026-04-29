# Anthropic Codex Strict Stream Plan

## 目标

为 `Codex` 渠道新增一条隔离的 Anthropic 兼容入口：

- 路径：`/anthropic/v1/messages`
- 目标客户端：`Claude Code` 及其他对 Anthropic streaming event 顺序要求严格的客户端
- 目标能力：允许客户端使用 Anthropic Messages 协议访问 `Codex` 模型

该方案的第一原则是：

- 不破坏现有 `/v1/messages`
- 不破坏现有 `/v1/responses` 与 `/v1/responses/compact`
- 不改变现有非 Codex 渠道行为

## 设计原则

1. 新路径完全隔离
   `Claude Code` 的严格兼容需求不继续叠加在现有 `/v1/messages` 上，而是单独落到 `/anthropic/v1/messages`。

2. 上游协议不变
   `Codex` 仍然只使用现有 `Responses` 上游调用方式，不新增新的 Codex 上游协议。

3. 下游输出单独实现
   新路径不直接复用现有通用 Claude 流式输出逻辑作为最终交付标准，而是单独实现严格 Anthropic SSE 渲染器。

4. 复用已有中间转换
   能复用的部分继续复用：
   - `Claude -> OpenAI request`
   - `Chat Completions -> Responses`
   - `Codex Responses upstream request`
   - 现有计费、日志、渠道分发、鉴权

5. 影响范围最小化
   所有 Codex + Claude Code 的特化逻辑仅在新路径内部生效，不反向污染现有公共路径。

6. 基础能力统一复用
   新路径不重新设计鉴权、限流、分发、计费、日志、状态码映射等基础能力，这些能力全部继续复用现有统一基础设施。

## 范围边界

本方案只解决一件事：

- `Codex Responses` 到 `Anthropic Messages` 的严格协议转换

本方案明确不重新设计以下基础能力：

- 鉴权
- 令牌校验
- 分组与渠道分发
- 配额预扣、结算、退款
- 使用日志与错误日志
- 全局限流
- 状态码映射
- 公共中间件行为

这些能力一律复用现有系统实现。
如果后续这些基础能力有问题，应在原有统一基础设施层修复，而不是在新路径中单独分叉。

## 路由方案

新增路由：

- `POST /anthropic/v1/messages`

路由行为：

- 请求体按 Anthropic `Messages` 协议解析
- 鉴权、分发、限流、配额、日志沿用现有 relay 基础设施
- 仅在该路径下进入新的严格流式处理链

现有路径保持不变：

- `POST /v1/messages`
- `POST /v1/responses`
- `POST /v1/responses/compact`

## 请求处理链

`/anthropic/v1/messages` 的处理链固定为：

1. 解析 Anthropic Messages 请求
2. 生成 `RelayInfo`
3. 渠道分发
4. 校验目标渠道是否为 `Codex`
5. 将 Claude 请求转换为 OpenAI Chat 请求
6. 将 OpenAI Chat 请求转换为 Responses 请求
7. 对发往 Codex 的 Responses 请求强制设置 `stream=true`
8. 调用 `Codex` 上游 `/backend-api/codex/responses`
9. 使用新的严格 Anthropic SSE 渲染器输出事件流
10. 复用现有 usage、计费与日志落账

如果目标渠道不是 `Codex`：

- 直接拒绝，返回清晰错误
- 不让新路径退回公共 `/v1/messages` 逻辑

这样可以保证新路径的语义明确，不引入“同一路径对不同渠道行为不同”的灰区。

## 唯一新增职责

新路径唯一新增职责只有三层转换：

1. 请求转换
   `Anthropic Messages -> OpenAI Chat -> OpenAI Responses`

2. 流事件转换
   `Codex Responses stream -> Anthropic strict SSE`

3. 终止语义转换
   `Responses terminal status / finish reason / stream end -> Anthropic message_delta / message_stop / error`

除这三层转换外，不新增其他业务职责。

## 严格 SSE 输出方案

新增一套专用于新路径的严格 Anthropic 流式渲染器，目标是对齐 Claude Code 对事件流的预期。

必须严格控制以下主事件顺序：

1. `message_start`
2. `content_block_start`
3. `content_block_delta`
4. `content_block_stop`
5. `message_delta`
6. `message_stop`

实现要求：

- 每个 `content_block_start` 必须有对应的 `content_block_stop`
- `index` 必须稳定递增且不可错乱复用
- `thinking`、`text`、`tool_use` 不能混用错误的 block type
- `stop_reason` 只在 `message_delta` 阶段输出，不能出现在其他事件中
- `usage` 只在 `message_delta` 阶段输出，并且必须视为累计值
- 最终必须保证流收尾完整

除主事件流外，还必须支持以下规范行为：

- `ping` 事件
  - 可在任意阶段穿插出现
  - 不能破坏主事件顺序状态机

- `error` 事件
  - SSE 在 HTTP 200 已建立后仍可能发出 `event: error`
  - 必须保证该事件能被下游客户端识别为流内错误，而不是被吞掉或错误包装成普通 chunk

- `message_delta` 可出现一次或多次
  - 不能假设流中只会有一次 `message_delta`
  - 必须允许累计 usage 和 top-level message 字段逐步变化

- 未知事件兼容
  - Anthropic 文档明确允许未来新增新的 event type
  - 新路径实现必须对未知上游事件和未来扩展预留忽略或透传策略，不能把未知事件直接当成协议错误导致整条流中断

- `message_start` 中 `content` 必须为空
  - 不能提前把第一段文本塞进 `message_start`

- 每个 content block 必须满足完整闭环
  - `content_block_start -> one or more content_block_delta -> content_block_stop`
  - 不允许出现未 start 先 delta、未 stop 即 message_stop、或 block type 与 delta type 不匹配

- `stop_reason` 的成功语义必须与错误语义分离
  - 正常结束依赖 `message_delta.stop_reason`
  - 错误场景依赖 `event: error`
  - 不能用 stop_reason 伪装错误，也不能用错误事件替代正常 stop

禁止继续沿用“尽量像 Anthropic”的宽松标准。
该路径的标准必须提升为“Claude Code 可稳定消费”。

## Anthropic Event Mapping Matrix

本节定义 `/anthropic/v1/messages` 的最终输出协议。后续实现必须按此矩阵落地，不能只做“看起来像 Anthropic”。

### 1. message_start

输出时机：

- 下游流开始时，且只能出现一次

必填字段：

- `type = "message_start"`
- `message.id`
- `message.type = "message"`
- `message.role = "assistant"`
- `message.model`
- `message.content = []`

可填字段：

- `message.usage.input_tokens`
- `message.usage.output_tokens`

严格要求：

- `message.content` 必须为空数组
- 不能把第一段文本、thinking、tool_use 提前塞进 `message_start`
- 一旦 `message_start` 发出，后续所有 block 只能通过 `content_block_*` 追加

Codex 映射来源：

- `response.id` 或网关生成的响应 ID
- `responses output model`
- 预估输入 tokens 可先写入 `input_tokens`

### 2. content_block_start

输出时机：

- 每开启一个新的 Anthropic content block 时输出

支持的 block type：

- `text`
- `thinking`
- `tool_use`

必填字段：

- `type = "content_block_start"`
- `index`
- `content_block.type`

按 block type 的附加要求：

- `text`
  - `content_block.text = ""`
- `thinking`
  - `content_block.thinking = ""`
- `tool_use`
  - `content_block.id`
  - `content_block.name`
  - `content_block.input = {}`

严格要求：

- 每个 `index` 的 block type 一旦开始，直到 stop 前不能变
- 同一个 `index` 不能复用成另一种 block type

Codex 映射来源：

- `output_text delta`
- `reasoning / summary delta`
- `function_call`

### 3. content_block_delta

输出时机：

- 对已经 start 的 block 追加内容时输出，可出现多次

必填字段：

- `type = "content_block_delta"`
- `index`
- `delta.type`

允许的 delta.type：

- `text_delta`
- `thinking_delta`
- `input_json_delta`

delta.type 对应约束：

- `text_delta`
  - 只能用于 `text` block
  - 必须带 `delta.text`

- `thinking_delta`
  - 只能用于 `thinking` block
  - 必须带 `delta.thinking`

- `input_json_delta`
  - 只能用于 `tool_use` block
  - 必须带 `delta.partial_json`

严格要求：

- 不允许 `text` block 输出 `thinking_delta`
- 不允许 `tool_use` block 输出 `text_delta`
- 不允许未 start 的 block 直接输出 delta

Codex 映射来源：

- Responses 的文本片段
- Responses 的 reasoning / summary 片段
- Responses 的 function arguments 片段

### 4. content_block_stop

输出时机：

- 一个 block 的所有 delta 输出完毕后

必填字段：

- `type = "content_block_stop"`
- `index`

严格要求：

- 每个 `content_block_start` 必须对应一个 `content_block_stop`
- `message_stop` 之前必须关闭所有未关闭 block

### 5. message_delta

输出时机：

- 一个 message 的顶层状态变化时输出，可出现一次或多次

必填字段：

- `type = "message_delta"`
- `delta`

允许字段：

- `delta.stop_reason`
- `delta.stop_sequence`
- `usage.input_tokens`
- `usage.output_tokens`

严格要求：

- `stop_reason` 只能出现在 `message_delta`
- `usage` 必须视为累计值，不是增量值
- 如果流中有多次 `message_delta`，后一次 `usage` 不能回退

建议 stop_reason 映射：

- 普通文本结束：`end_turn`
- 工具调用结束：`tool_use`
- 被长度截断：`max_tokens`
- 命中停止序列：`stop_sequence`

Codex 映射来源：

- Responses completion status
- Responses usage
- OpenAI / Responses finish reason 规范化映射

### 6. message_stop

输出时机：

- 流正常结束时，且只能出现一次

必填字段：

- `type = "message_stop"`

严格要求：

- 必须在所有 block stop 和最终 `message_delta` 之后输出
- 输出后不得再继续输出 `content_block_*` 或 `message_delta`

### 7. ping

输出时机：

- 作为 keepalive，可在任意主流程阶段穿插

必填字段：

- `type = "ping"`

严格要求：

- `ping` 不能改变当前 block 状态
- `ping` 不能替代 `message_delta` 或 `message_stop`

### 8. error

输出时机：

- HTTP 200 已建立之后，流内发生无法恢复的错误时

必填字段：

- `type = "error"`
- `error.type`
- `error.message`

建议字段：

- `error.request_id`
- `error.details`

严格要求：

- `error` 事件和普通完成事件不能同时作为最终状态
- 已输出 `error` 后，不再继续输出正常 `message_stop`

## Codex To Anthropic Field Mapping

本节限定从 Codex / Responses 到 Anthropic 的字段来源，避免后续实现时出现多套不一致映射。

- Anthropic `message.id`
  - 来源：Codex response id；若缺失，使用网关生成值

- Anthropic `message.model`
  - 来源：最终上游模型名

- Anthropic `usage.input_tokens`
  - 来源：Responses usage；若上游首包缺失，可先使用预估值，最终必须在 `message_delta` 中纠正为累计真实值

- Anthropic `usage.output_tokens`
  - 来源：Responses usage

- Anthropic `text block`
  - 来源：Responses output text

- Anthropic `thinking block`
  - 来源：Responses reasoning summary / reasoning text
  - 如果 Codex 不稳定提供 reasoning，则实现必须允许不输出 thinking block，而不是输出半残结构

- Anthropic `tool_use`
  - 来源：Responses function call

- Anthropic `input_json_delta.partial_json`
  - 来源：function arguments 的增量片段

- Anthropic `stop_reason`
  - 来源：Responses terminal status / finish reason 映射

## Claude Code Compatibility Requirements

如果目标是“Claude Code 中全面使用 Codex API”，则以下要求全部视为硬约束：

1. 只能输出 Anthropic 命名 SSE event
   不能输出 OpenAI chunk 形式的 data masquerade。

2. 必须严格闭合 block
   `text`、`thinking`、`tool_use` 任何一种 block 都不能出现 start / delta / stop 缺失。

3. 顶层 message 生命周期完整
   必须有 `message_start`，最终必须有正常 `message_stop` 或流内 `error`，不能只断连接。

4. usage 与 stop_reason 位置正确
   只能出现在 `message_delta`。

5. 未知事件前向兼容
   不因为上游新增事件就直接断流。

6. 客户端断流与上游断流可区分
   必须能区分：
   - Claude Code 主动取消
   - Codex 上游异常中止
   - 网关自身渲染失败

## Forward Compatibility Strategy

Anthropic 文档明确要求客户端具备前向兼容能力，因此新路径实现也必须具备前向兼容策略。

对未来未知事件的统一策略：

1. 网关内部未知上游事件
   - 记录调试日志
   - 不直接终止整条流
   - 若无法映射到 Anthropic 规范事件，则忽略并保留状态机

2. 未知 Anthropic 目标事件
   - 第一版不主动生成
   - 保留扩展点，不把当前 event switch 写成 panic / fatal default

3. 未知 delta 子类型
   - 记录日志
   - 不污染当前 block type
   - 不自动降级成错误 delta

## Non-Goals

以下内容不属于本方案第一版目标，必须明确排除，避免范围失控：

- 让现有 `/v1/messages` 直接兼容 Claude Code
- 让所有非 Codex 渠道自动继承新严格流式实现
- 在第一版中完整兼容所有复杂 MCP / tool streaming 变体
- 通过修改公共 Claude 转换器去“顺带”修复 Claude Code
- 以牺牲现有 `/v1/messages` 兼容行为换取 Claude Code 可用

## 代码边界

建议新增以下边界：

- 新路由注册
  - 位置：`router/relay-router.go`

- 新 handler
  - 建议文件：`relay/anthropic_strict_handler.go`
  - 仅负责 `/anthropic/v1/messages`

- 新流式渲染器
  - 建议文件：`relay/channel/openai/anthropic_strict_stream.go`
  - 或 `relay/anthropic_strict_stream.go`

- 新非流式转换器
  - 建议文件：`service/anthropic_strict_response.go`
  - 用于未来需要补齐非流式严格输出时统一处理

已有可复用代码：

- `service.ClaudeToOpenAIRequest`
- `service.ChatCompletionsRequestToResponsesRequest`
- `channel/codex.Adaptor.ConvertOpenAIResponsesRequest`
- `channel/codex.Adaptor.DoRequest`
- 现有 usage / quota / billing / log 处理逻辑

不建议直接修改的公共区域：

- 现有 `/v1/messages` 默认流式输出逻辑
- 现有 `service.StreamResponseOpenAI2Claude` 公共逻辑
- 现有普通 `Responses` 路径的流终止与补尾逻辑

## 兼容策略

新路径第一期只承诺以下能力：

- 普通文本对话
- 流式文本返回
- 基础 usage 落账

第一期不强承诺：

- `thinking` 完整语义
- `tool_use` / `tool_result`
- 多 block 并发复杂场景
- 所有第三方 Anthropic 客户端完全兼容

如果第一期的目标客户端包含 `Claude Code`，则以下能力不能跳过：

- `text` block 的严格顺序
- 至少一种稳定的 `stop_reason`
- `ping` / `error` 事件处理
- `message_delta` 与 `message_stop` 的正确收尾

如果第一期就要兼容 Claude Code，则 Claude Code 的真实交互必须纳入验收标准，而不是只依赖 curl 或通用 SDK。

## 转换层异常策略

本节只定义转换层异常，不重新定义系统基础错误处理。

新路径必须显式处理以下转换层异常：

- Claude 请求成功解析，但无法转换为 OpenAI Chat
- OpenAI Chat 成功生成，但无法转换为 Responses
- Responses 上游流存在事件缺口，无法形成合法 Anthropic block 生命周期
- 上游返回的 delta 类型与当前 block type 冲突
- 上游流提前结束，但未形成合法 Anthropic terminal
- 上游返回未知事件或未知 delta 子类型
- Anthropic 严格 SSE 渲染器内部状态机冲突

新路径不负责重新定义以下基础异常：

- 鉴权失败
- 额度不足
- 渠道分发失败
- 基础日志落库失败
- 全局限流命中

这些统一沿用原系统已有处理方式。

转换层要求：

- 不能把转换异常静默吞掉
- 不能把所有转换失败都折叠成通用 `context canceled` 或 `client_gone`
- 必须能区分：
  - 转换前失败
  - 转换中失败
  - 上游流不完整
  - 下游客户端主动取消

建议在新路径中增加仅服务于转换层的流状态埋点：

- upstream connected
- first convertible event received
- anthropic message_start emitted
- anthropic block start emitted
- anthropic block stop emitted
- anthropic terminal emitted
- downstream canceled before terminal

## 验收标准

功能验收必须按以下顺序进行：

1. `curl` 调用 `/anthropic/v1/messages`，文本流式返回完整
2. 非流式 Anthropic 请求能返回完整 `message` JSON
3. `Codex` 上游强制 `stream=true` 时不再出现 `Stream must be set to true`
4. 完整 usage、日志、计费不丢失
5. 现有 `/v1/messages` 不回归
6. 现有 `/v1/responses` 不回归
7. Claude Code 实际接入成功，完成至少一轮稳定对话

只有第 7 条通过，才能定义为“支持 Claude Code”。

## 测试范围

必须新增定向测试：

- 新路径只允许 `Codex`
- 新路径内部转发到 `Responses` 时强制 `stream=true`
- 新路径的 SSE 事件顺序正确
- 新路径收尾必须输出 `message_stop`
- 新路径不影响现有 `/v1/messages`
- 新路径不影响现有 `/v1/responses`

建议增加录制型测试或事件快照测试，避免后续再被公共逻辑回归破坏。

建议新增规范对照测试：

- `message_start.content` 为空
- `message_delta.stop_reason` 只在 message_delta 中出现
- `usage` 在 message_delta 中为累计值
- `ping` 事件不会破坏状态机
- `error` 事件在流中能被正确输出
- 未知事件不会直接导致整条流异常终止

## 最终结论

推荐方案是：

- 新增 `/anthropic/v1/messages`
- 专门服务 `Codex + Claude Code`
- 上游继续走 `Responses`
- 下游单独实现严格 Anthropic streaming event
- 与现有 `/v1/messages` 完全隔离

这是当前最符合“最小回归风险”和“可持续维护”的实现路线。

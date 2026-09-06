# Seedance 2.0 测试与方案完成性审计

审计时间：2026-07-11（Asia/Shanghai）

## 一、用户要求与证据

| 要求 | 状态 | 权威证据 |
|---|---|---|
| 测试 `videos-fast` | 完成 | 480p、720p 均完成并保存成片，见 `cases/videos-fast_*` |
| 测试 `videos-mini` | 完成 | 480p、720p 均完成并保存成片，见 `cases/videos-mini_*` |
| 测试 `videos-standard` | 完成 | 480p、720p、1080p 完成；4k 两次进入任务后失败并全额退款 |
| 覆盖所有 8 个价格点 | 完成 | `manifest.json`、`run_summary.json`、`billing_summary.json` |
| 按次档位使用 15 秒 | 完成 | 六个按次用例的 `input.json` 均为 `duration: 15` |
| 按秒档位测试约 2 秒 | 完成边界测试 | 1080p、4k 的 2 秒请求均 HTTP 422 且零扣费；官方最短为 4 秒，因此又以 4 秒完成有效计费测试 |
| 所有请求并发，不逐个串行 | 完成 | 其余 7 个初始用例于 18:12:54 同秒提交；4 秒替代用例于 18:13:50 同秒提交，此时 6 个按次任务全部仍在运行，8 个有效任务完整重叠 |
| 保存所有输入参数 | 完成 | 每个用例均有 `input.json`，认证头统一为 `<REDACTED>` |
| 保存所有输出参数 | 完成 | 每个用例均有 `submit_response.json`、`final_response.json`、`summary.json`、额度快照；异步任务还保存全部 `poll_*.json` |
| 保存成功视频并验证规格 | 完成 | 7 个 `output.mp4` 和对应 `output_ffprobe.json` |
| 使用图片、视频、音频参考参数 | 完成协议测试 | 首批有效用例均携带三个 `reference*` 数组并被接口接受；精确语义遵循程度无法仅凭协议响应定量证明 |
| 对照 Seedance 2.0 官方参数 | 完成 | `REPORT.md` 和 `official_seedance_request_example.json` |
| 评估 New API 模型、计费和任务集成 | 完成方案 | `INTEGRATION_PLAN.md` |
| 重新核实现有失败退款技术方案 | 完成 | 源码链路审计、两次 4k 全额退款在线证据、`INTEGRATION_PLAN.md` 第一和第七节 |
| 未经确认不修改生产代码 | 完成 | 本任务只新增或更新 `test-results/seedance2-20260711` 下的测试、视频和文档产物 |

## 二、计费核对

八个有效价格点的预期总额：

```text
2.00 + 3.50 + 1.50 + 2.50 + 3.50 + 5.00 + 0.60×4 + 1.20×4
= ¥25.20
```

在线首次总预扣：

```text
12,600,000 quota ÷ 500,000 quota/元 = ¥25.20
```

4k 失败退款：

- 完整参考素材用例：预扣 ¥4.80，退款 ¥4.80。
- 安全文本重试用例：预扣 ¥4.80，退款 ¥4.80。

最终净扣：

```text
10,200,000 quota ÷ 500,000 quota/元 = ¥20.40
```

七个成功档位价格之和同样为 ¥20.40，因此没有发现漏扣、重复扣费或失败未退款。

## 三、并发执行核对

`run_summary.json` 记录：

| 时间 | 操作 |
|---|---|
| 18:05:30 | fast 480p 提交，持续运行至 18:35:46 |
| 18:12:54 | fast 720p、mini 480p/720p、standard 480p/720p、1080p 2s、4k 2s 同秒并发提交 |
| 18:13:50 | 2 秒被拒后，1080p 4s、4k 4s 同秒提交 |

18:13:50 时六个按次任务均未完成，因此八个有效计费任务同时运行。执行脚本的 `--parallel` 路径使用 `ThreadPoolExecutor(max_workers=len(selected_cases))`。

## 四、产物完整性

### 成功任务

以下 7 个目录均包含请求、创建响应、全部轮询响应、最终响应、额度快照、MP4 和 ffprobe：

- `videos-fast_480p_15s`
- `videos-fast_720p_15s`
- `videos-mini_480p_15s`
- `videos-mini_720p_15s`
- `videos-standard_480p_15s`
- `videos-standard_720p_15s`
- `videos-standard_1080p_4s_retry`

### 失败任务

以下目录包含请求、创建响应、全部轮询响应、最终失败响应和退款前后额度快照：

- `videos-standard_4k_4s_retry`
- `videos-standard_4k_4s_safe_retry`

### 预扣前拒绝任务

以下目录包含请求、HTTP 422 响应、最终摘要和零额度变化证据：

- `videos-standard_1080p_2s`
- `videos-standard_4k_2s`

### 完整性检查结果

- 所有 JSON 产物均通过 `jq empty` 解析检查。
- 7 个 MP4 均可读取并已记录 SHA-256。
- 7 个 MP4 均通过 ffprobe；视频轨和 AAC 音轨可识别。
- 测试 Key 只从 `NEWAPI_API_KEY` 环境变量读取，保存的请求头均为 `Bearer <REDACTED>`。
- 源码基线命令 `go test ./relay/common ./relay/channel/task/sora ./service ./model` 通过；Sora 包当前显示 `no test files`，因此集成方案明确要求新增价格矩阵和参数校验测试。

## 五、方案审计结论

正常任务链路已覆盖预扣、提交失败退款、终态失败退款、超时退款和 CAS 防重复退款，不应重写 `PriceData`、`BillingSession`、`RefundTaskQuota` 或 Task CAS。

最小模型集成应保持三个公开模型，通过 Sora/OpenAI task adaptor 解析 `resolution` 和 `duration`，使用现有 `OtherRatios` 表达六个按次价格和两个按秒价格。

生产启用前还需单独修复两条通用异常路径：

1. 轮询时渠道读取失败当前会无 CAS 标记失败且不退款。
2. 上游创建成功后，任务插入或结算失败可能留下扣费与任务记录不一致。

这两项只需要复用现有 CAS 和退款函数，不需要新的退款系统。完整逐文件方案见 `INTEGRATION_PLAN.md`。

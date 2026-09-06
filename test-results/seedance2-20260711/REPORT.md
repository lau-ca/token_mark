# Seedance 2.0 兼容视频接口测试报告

测试时间：2026-07-11（Asia/Shanghai）  
接口：`https://newapi.megabyai.cc/v1/videos`  
认证信息：已脱敏，测试产物中未保存 API Key。

## 结论摘要

- 在线 `/v1/models` 已公开 `videos-fast`、`videos-mini`、`videos-standard`，并同时公开相应分辨率后缀模型。
- 6 个按次计费档位均以 15 秒并发运行成功；其余 2 个有效按秒档位在这些任务仍运行时同时提交，因此 8 个有效计费任务存在完整并发重叠，不是逐个串行测试。
- `videos-standard` 的 1080p、4k 以 2 秒提交时均返回 HTTP 422，且未扣费；改为 Seedance 2.0 官方最短 4 秒后均提交成功。
- 8 个有效档位的首次总预扣为 `12,600,000 quota`。站点 `quota_per_unit=500,000`、`quota_display_type=CNY`、`usd_exchange_rate=1`，折算正好为 ¥25.20。
- 7 个档位生成成功并下载成片；4k 在两次不同输入下都于 89% 失败，错误均为“内容未通过社区规范审核”。两次 4k 预扣均准确退回 ¥4.80。
- 最终净扣费为 `10,200,000 quota = ¥20.40`，正好等于 7 个成功档位价格之和。
- 三种模型的成功成片均明显保留参考苹果图片主体，且全部含 AAC 音轨；兼容字段已通过实测。参考视频/音频的精确语义遵循程度无法仅凭协议响应定量证明。
- `/content` 链路存在缺陷：任务查询先返回 completed，但内容接口短暂仍读到 IN_PROGRESS；随后 `/content` 可能 502 或 302 到明文 HTTP IP。上游 `apisd` 直链只有携带 Range 时稳定返回 206 MP4。

## 测试矩阵

| 用例 | 时长 | 预期价格 | 提交结果 | 任务 ID |
|---|---:|---:|---|---|
| videos-fast / 480p | 15s | ¥2.0/次 | completed，成片已保存 | `videos-fast_a1daa2ec754a492f8bd6af2b0f3aa2d5` |
| videos-fast / 720p | 15s | ¥3.5/次 | completed，成片已保存 | `videos-fast_582974a3e7ab4071bc05036debd4e21b` |
| videos-mini / 480p | 15s | ¥1.5/次 | completed，成片已保存 | `videos-mini_c1755a91cc32458797897c0e75a9a124` |
| videos-mini / 720p | 15s | ¥2.5/次 | completed，成片已保存 | `videos-mini_9731d5f3e5ec4af2a60e0be70cb25e62` |
| videos-standard / 480p | 15s | ¥3.5/次 | completed，成片已保存 | `videos-standard_6c9cf5fd68e3415c8e136e72ba5ec8fc` |
| videos-standard / 720p | 15s | ¥5.0/次 | completed，成片已保存 | `videos-standard_ba855a1d62554deea00156dc16b17005` |
| videos-standard / 1080p | 2s | ¥1.2 | HTTP 422，无任务、无扣费 | - |
| videos-standard / 4k | 2s | ¥2.4 | HTTP 422，无任务、无扣费 | - |
| videos-standard / 1080p | 4s | ¥2.4 | completed，成片已保存 | `videos-standard_916b02d7e4d84dfa9fef3492569d0026` |
| videos-standard / 4k（完整参考素材） | 4s | ¥4.8 | failed@89%，全额退款 | `videos-standard_8a66760f81214dec8246dd91e8170f51` |
| videos-standard / 4k（安全文本重试） | 4s | ¥4.8 | failed@89%，全额退款 | `videos-standard_55d1b9d209e74ffaab2dd68bc85e7689` |

## 并发执行证据

- `videos-fast / 480p` 于 18:05:30 提交，并持续运行至 18:35:46。
- 其余 5 个按次档位和 2 个 2 秒边界用例于 18:12:54 同秒并发提交。
- 2 秒请求被拒绝后，1080p、4k 的 4 秒有效请求于 18:13:50 同秒提交；此时 6 个按次任务均未完成。
- 因此从 18:13:50 起，6 个按次任务与 2 个按秒任务共 8 个有效计费任务同时处于运行状态。

精确提交时间保存在 `run_summary.json`，并发执行代码位于 `run_tests.py` 的 `ThreadPoolExecutor` 分支。

## 成片规格验证

| 用例 | 实际视频 | 实际时长 | 音轨 |
|---|---|---:|---|
| fast / 480p | H.264，864×496 | 15.093s | AAC |
| fast / 720p | H.264，1280×720 | 15.093s | AAC |
| mini / 480p | H.264，864×496 | 15.104s | AAC |
| mini / 720p | H.264，1280×720 | 15.104s | AAC |
| standard / 480p | H.264，864×496 | 15.069s | AAC |
| standard / 720p | H.264，1280×720 | 15.069s | AAC |
| standard / 1080p | H.264，1920×1080 | 4.063s | AAC |

480p 档位实际输出高度为 496，而不是严格 480；应按上游实际规格对外说明。4k 两次均未产生成片，无法验证官方所述的 HEVC 10-bit 输出。

## 实际测试输入

首批 8 个有效档位都包含以下字段：

- `model`
- `prompt`
- `duration`
- `ratio: "16:9"`
- `resolution`
- `referenceImages`
- `referenceVideos`
- `referenceAudios`

参考素材使用火山方舟 Seedance 2.0 官方教程资源：

- 图片：`https://ark-project.tos-cn-beijing.volces.com/doc_image/r2v_tea_pic1.jpg`
- 视频：`https://ark-project.tos-cn-beijing.volces.com/doc_video/r2v_tea_video1.mp4`
- 音频：`https://ark-project.tos-cn-beijing.volces.com/doc_audio/r2v_tea_audio1.mp3`

每个用例的完整请求、提交响应、所有轮询响应和最终响应均保存在 `cases/<用例名>/` 下。

4k 第二次重试仅使用安全文本提示词，不携带参考素材，用于排除素材/提示词导致的单次审核问题；结果仍在 89% 失败。

## 与 Seedance 2.0 官方接口的差异

用户示例不是火山方舟原生格式，而是第三方兼容格式：

- 官方入口：`POST /api/v3/contents/generations/tasks`，不是 `/v1/videos`。
- 官方模型为 `doubao-seedance-2-0-*`，`videos-*` 是站点别名。
- 官方素材通过 `content[]` 中的 `image_url`、`video_url`、`audio_url` 和对应 `role` 传入，不使用顶层 `referenceImages/referenceVideos/referenceAudios`。
- 官方提示词引用使用 `图片1`、`视频1`、`音频1`；`@image1/@video1/@audio1` 不是火山官方保证的写法。
- 官方三个模型的输出时长均为整数 4～15 秒或 `-1`，本次 2 秒请求被在线接口拒绝与该限制一致。

官方来源：

- https://www.volcengine.com/docs/82379/1520757
- https://www.volcengine.com/docs/82379/2291680
- https://www.volcengine.com/docs/82379/1521309

站点公开 Apifox 文档只声明 `videos`、4～15 秒、720p、`referenceImages/referenceVideos`，未声明三个 `videos-*` 别名、480p/1080p/4k 或 `referenceAudios`；在线实际能力已超出公开文档，文档需要同步。

## New API 集成方案

当前任务预扣、正常提交失败退款、任务终态退款、超时退款和 CAS 防重复退款框架可以复用；不应为三个模型重写计费核心。需要开发的是公开参数归一化和按分辨率的混合计费倍率。

推荐方案：

1. 根据实际渠道选择只改一个适配器：OpenAI/Sora 直通渠道，或 DoubaoVideo 官方渠道。
2. 统一解析并验证 `duration`、`ratio`、`resolution` 和三个 `reference*` 数组；禁止通过 `metadata` 绕过时长上限。
3. 使用各模型 480p 价格作为 `ModelPrice` 基准，按站点汇率写入内部 USD 价格。
4. 在适配器 `EstimateBilling` 中返回倍率：
   - fast：480p `1`，720p `1.75`
   - mini：480p `1`，720p `5/3`
   - standard：480p `1`，720p `10/7`，1080p `(6/35) × duration`，4k `(12/35) × duration`
5. 复用现有 `OtherRatios`、预扣、结算和失败退款，不改核心 `PriceData`、`BillingSession`、`RefundTaskQuota` 和 Task CAS。
6. 若使用 DoubaoVideo 适配器，把顶层 `referenceImages/Videos/Audios` 转为官方 `content[]`，并让顶层 `duration/resolution/ratio` 明确覆盖旧 `metadata`。
7. 单独修复两个通用任务异常路径：轮询时渠道读取失败必须经 CAS 退款；任务插入或结算失败不得留下净扣费。
8. 任务状态一致性、Range/206 内容代理和 HTTPS 安全重定向作为独立补丁，不与计价修改混合。
9. 将 4k 89% 稳定失败作为独立上游问题排查，记录真实供应商错误而不是统一包装成社区规范审核。
10. 增加 8 个价格点、非法模型/分辨率、2 秒时长、参数转换和异常退款路径的确定性测试。

完整开发前方案见 `INTEGRATION_PLAN.md`。

## 产物结构

- `manifest.json`：全部用例与参考素材清单
- `environment_models.json`：在线模型响应
- `environment_status.json`：站点配置响应
- `environment_usage_final_raw.json`：测试结束后的原始额度响应
- `billing_summary.json`：预扣、退款和最终净扣费汇总
- `COMPLETION_AUDIT.md`：用户要求到测试证据的逐项完成性审计
- `official_seedance_request_example.json`：火山官方原生请求对照
- `cases/<用例>/input.json`：脱敏后的完整请求
- `cases/<用例>/submit_response.json`：完整提交响应与响应头
- `cases/<用例>/poll_*.json`：每次查询响应
- `cases/<用例>/final_response.json`：最终响应
- `cases/<用例>/output.*`：成功后下载的成片
- `cases/<用例>/output_ffprobe.json`：成片编码、分辨率和时长验证
- `cases/<用例>/content_download_attempt_*.json`：New API 内容代理尝试
- `cases/<用例>/direct_content_download_attempt_*.json`：上游 Range 直链下载尝试

# 百度千帆视频渠道接入设计

## 目标

在现有百度千帆 V2 渠道类型内增加异步视频任务支持，使平台统一的 OpenAI Video 接口可以调用百度千帆的 `K3.0` 图生视频和 `k3.0-turbo` 文生视频能力，同时不改变任何其他渠道的请求、轮询、计费或响应行为。

## 范围

本次接入仅覆盖：

- `K3.0` 图生视频。
- `k3.0-turbo` 文生视频。
- 创建任务、后台轮询、任务状态查询和成功结果中的视频 URL。
- 平台已有的模型价格、分组倍率、预扣费和任务结算链路。

不覆盖千帆文档中的 OMNI、动作控制、人脸识别、对口型、主体或自定义音色接口，也不改变现有百度千帆 V2 的聊天、嵌入、图像和重排路径。

## 渠道隔离

复用 `constant.ChannelTypeBaiduV2`，在任务适配器选择处仅为该渠道类型返回新的千帆视频适配器。新的实现放在独立的 `relay/channel/task/qianfan` 包中。

OpenAI、Sora、可灵、Seedance、火山、阿里等现有任务适配器不共享或调用千帆专用转换逻辑。这样千帆协议变化只影响百度千帆 V2 渠道。

## 对外接口

客户端继续使用平台已有接口：

- `POST /v1/videos` 创建视频任务。
- `GET /v1/videos/{task_id}` 查询任务。
- `GET /v1/videos/{task_id}/content` 通过平台视频代理获取成功结果。

请求沿用 `relay/common.TaskSubmitReq`：

- 公共字段：`model`、`prompt`、`duration`、`seconds`。
- 图生视频：`image`。
- 文生视频：`resolution`、`aspect_ratio`。
- `metadata` 仅用于承载千帆确实支持、且已经过本地校验的补充参数；不得覆盖模型和计费相关字段。

## 上游协议转换

### 图生视频

当模型映射后的上游模型为 `K3.0` 时：

- 请求地址：`POST {base_url}/beta/video/generations/qianfan-video`。
- `type` 固定为 `img2video`。
- 将平台的 `image`、`prompt` 和时长转换到 `model_parameters`。
- 图片为空时返回 400，不向上游发送请求。

### 文生视频

当模型映射后的上游模型为 `k3.0-turbo` 时：

- 请求地址：`POST {base_url}/beta/video/generations/qianfan-video`。
- `type` 固定为 `text2video`。
- 将平台的 `prompt` 放入 `model_parameters.prompt`。
- 将 `resolution`、`aspect_ratio` 和时长放入 `model_parameters.settings`。

模型判断使用大小写不敏感比较，但发送给上游时保留渠道模型映射给出的名称，避免改变管理员配置。

## 鉴权与 URL

渠道 Base URL 默认继续使用 `https://qianfan.baidubce.com`。适配器统一去除尾部 `/`，再追加固定路径，避免出现双斜杠。

创建和查询均使用：

```text
Authorization: Bearer <渠道密钥>
Content-Type: application/json
```

轮询地址为：

```text
GET {base_url}/beta/video/generations/qianfan-video?task_id={upstream_task_id}&model={upstream_model}
```

查询参数必须通过 `net/url` 编码，不手工拼接用户或上游值。

## 响应与状态

创建响应读取 `data.task_id`，平台仍向客户端返回公开的 `task_xxxx` ID，不暴露上游任务 ID。

轮询响应状态转换如下：

| 千帆状态 | 平台状态 |
|---|---|
| `submitted` | submitted |
| `processing` | in progress |
| `succeed` | success |
| `failed` | failure |

成功时读取 `data.task_result.videos[0].url`。任务对外响应使用平台统一的 OpenAI Video 结构；成功结果中的 URL 替换为平台 `/v1/videos/{public_task_id}/content` 代理地址，上游真实 URL 仅保存在任务内部数据中。

## 参数和计费安全

时长优先读取 `seconds`，其次读取 `duration`。缺失时不自行制造上游不保证支持的默认值；非法格式、非正数或超过 `relay/common.MaxTaskDurationSeconds` 时返回 400。

预扣费通过 `EstimateBilling` 添加经过校验的 `seconds` 倍率。其他倍率仅在明确对应现有价格配置时添加，不引入隐含价格。适配器嵌入 `taskcommon.BaseBilling`，提交或完成阶段不根据不可信的上游字段重新计算费用。

## 错误处理

- HTTP 非成功响应转为任务错误，并保留可诊断的上游错误信息给后台日志。
- 千帆返回 `code != 0` 时视为失败响应。
- 创建响应缺少 `data.task_id` 时返回无效上游响应。
- 轮询响应缺少已知状态时保持非终态，不误判成功。
- 成功状态缺少视频 URL 时不标记为可交付成功，避免产生空内容代理。

## 测试

新增确定性单元测试覆盖：

- 百度千帆 V2 被路由到新的任务适配器，其他现有渠道类型保持原适配器。
- 图生视频和文生视频的 URL、Header、请求 JSON 精确匹配。
- 模型映射后的名称被发送并用于查询参数。
- 时长边界、缺少图片和不支持模型返回 400。
- 创建响应提取嵌套 `data.task_id`。
- 四种千帆状态和成功视频 URL 的解析。
- 对外任务 ID 与上游任务 ID 隔离。

验证只运行相关 Go 包的单元测试，不执行端到端浏览器测试。

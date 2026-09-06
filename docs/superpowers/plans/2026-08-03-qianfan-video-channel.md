# 百度千帆视频渠道实施计划

## 目标

在百度 V2 渠道中独立接入千帆标准/高级视频任务和管理员主体音色资源接口，并用请求级表达式支持复杂计费，不改变其他渠道行为。

## 实施步骤

1. 新增 `relay/channel/task/qianfan` 的 DTO、校验和任务适配器测试，覆盖标准文生/图生、高级类型、上游响应、状态和 OpenAI Video 转换。
2. 实现千帆任务适配器：固定上游路径、Bearer 鉴权、模型映射、请求转换、任务查询和结果解析。
3. 在 `relay.GetTaskAdaptor` 中仅为 `ChannelTypeBaiduV2` 注册新适配器，并增加路由隔离测试。
4. 新增 `/qianfan/v1/videos`，复用 `controller.RelayTask`；高级请求在适配器内转换成已校验的 `TaskSubmitReq` 供计费和任务快照使用。
5. 扩展任务表达式的百度 V2 规范输入，仅暴露白名单字段 `operation/duration/mode/sound/resolution/has_reference_video/has_voice`，补充计费测试。
6. 后台轮询把保存的上游模型加入查询参数 map，补充测试证明已有 action/task_id 参数保持不变。
7. 新增管理员千帆资源代理 controller 和渠道权限路由，固定主体/音色操作路径并验证渠道类型；使用 `httptest` 覆盖请求方法、URL、鉴权和错误响应。
8. 运行定向测试：`go test ./relay/channel/task/qianfan ./relay ./relay/helper ./service ./controller ./router`；再运行 `gofmt`、`git diff --check` 并人工检查相关 diff，保留所有无关工作区改动。

# Playground 模型能力配置设计

## 目标

将现有 `/playground` 从仅支持文本对话扩展为复用同一页面框架的对话、图片和视频使用入口，并允许管理员决定每个模型向用户开放哪些参数和值。

## 约束

- 复用现有页面布局、UI 组件、模型管理页签和 Playground 消息存储。
- 不新增数据库表，不新增 `models` 表字段。
- 能力配置继续保存在现有 `models.endpoints` JSON 中。
- 不改变现有 `/api/user/models` 默认返回格式；详细能力通过可选查询参数返回。
- 不重写文本聊天链路；图片和视频作为现有 Playground 的并列模式接入。
- 参数限制必须由 Playground 后端校验，不能只依赖前端隐藏选项。

## 配置结构

`models.endpoints` 保持原有端点键结构，在端点对象中增加可选 `playground` 配置。已有只包含 `path` 和 `method` 的配置继续有效。

```json
{
  "image-generation": {
    "path": "/v1/images/generations",
    "method": "POST",
    "playground": {
      "capabilities": ["image.generate", "image.edit"],
      "parameters": [
        {
          "key": "size",
          "label": "Image size",
          "type": "enum",
          "default": "1024x1024",
          "options": ["1024x1024", "2048x2048"]
        }
      ]
    }
  }
}
```

首期参数类型限定为 `string`、`number`、`boolean`、`enum`。系统内置常用参数模板，管理员可以启用、关闭、调整默认值和允许值，也可以增加同类型的普通透传参数。`model`、鉴权、请求地址以及计费敏感字段的转换不允许由管理员自由改写。

## 管理页面

复用模型管理现有 `/models/$section` 页面，增加 `/models/capabilities` 页签。页面复用模型列表、表单、抽屉、按钮、输入框、选择器和开关组件。

能力编辑器按模型显示：

- 用户场景：对话、图片生成、图片编辑、文生视频、图生视频。
- 调用端点：OpenAI Chat、Responses、Anthropic、Gemini、OpenAI Images、OpenAI Video。
- 用户参数：是否展示、类型、默认值、允许值、最小值和最大值。
- 表单预览：展示 Playground 最终生成的控件。
- 高级 JSON：保留现有端点 JSON 编辑能力作为兜底。

## Playground

现有文本模式和本地消息存储保持不变。页面顶部增加复用现有 Tabs 的模式切换：对话、图片、视频。

- 对话继续使用现有 `PlaygroundChat`、`PlaygroundInput`、消息状态和存储键。
- 图片模式使用现有表单、文件输入、按钮、卡片和加载状态组件，调用新增的用户会话图片路由。
- 视频模式使用同一能力参数定义创建任务，并以任务卡片轮询状态。
- 模型列表由当前分组决定，只显示具备当前模式能力的模型。

## 后端路由

复用现有 Playground 临时 Token 上下文：

- `POST /pg/chat/completions` 保持不变。
- `POST /pg/images/generations` 转发到现有 OpenAI Image relay。
- `POST /pg/images/edits` 转发 multipart 图片编辑。
- `POST /pg/videos` 创建现有视频任务。
- `GET /pg/videos/:task_id` 查询任务。
- `GET /pg/videos/:task_id/content` 读取完成视频。

所有路由继续使用登录用户会话，不要求用户在浏览器暴露 API Key。

## 兼容性

- 没有 `playground` 配置的模型继续按现有端点推断文本能力。
- 原有 `endpoints` 字符串值和 `{path, method}` 对象继续支持。
- `/api/user/models` 未传详细参数时仍返回字符串数组。
- 现有 Playground 消息内容和配置存储不迁移、不清空。

## 验证

- Go 单元测试覆盖配置解析、模型能力过滤和 Playground 参数拒绝。
- 前端 Vitest 覆盖端点 JSON 解析、动态参数默认值和请求 Body 构建。
- 执行 Go 定向测试、前端 typecheck、涉及文件 lint 和生产构建。
- 按用户默认偏好不执行浏览器端到端测试。

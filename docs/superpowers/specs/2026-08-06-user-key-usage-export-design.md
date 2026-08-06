# 普通用户 Key 消耗 Excel 导出设计

## 目标

在新 UI 的 `/dashboard/keys` 页面为普通用户增加“导出报告”功能。报告按当前筛选时间范围统计当前账号下每个 API Key 的请求数、输入 Token、输出 Token、总 Token、消费金额、使用模型和最后调用时间，并生成精美、专业、可筛选的 Excel 工作簿。

## 页面与权限

- 导出按钮放在 Key 消耗分析页右上角，与“筛选”并列。
- 接口使用 `middleware.UserAuth()`，用户 ID 只取认证上下文，不接受前端传入其他用户 ID。
- 时间范围与现有 Key 消耗筛选完全一致，最长 30 天。
- 导出全部筛选结果，不受页面 Key 搜索、表格排序或分页影响。
- 不返回或导出完整 API Key，只使用 Key 名称、脱敏 Key 与 Key ID。

## 数据来源与口径

- 使用 `model.LOG_DB` 中的消费日志，因为日志同时保存 `token_id`、`token_name`、`model_name`、`prompt_tokens`、`completion_tokens`、`quota` 和 `created_at`。
- 仅统计 `LogTypeConsume`，并限制 `user_id` 为当前登录用户。
- 按 `token_id + model_name` 聚合模型明细；Key 汇总由模型明细汇总得到。
- 当前仍存在但选定范围内没有消费的 Key 也出现在 Key 汇总中，数值为零。
- 已删除 Key 的历史消费继续保留，名称显示为“已删除 Key（ID）”。
- 消费金额按当前系统币种配置把 quota 转为展示金额；工作簿同时保留原始 quota，便于核账。

## API

新增：

```text
GET /api/data/keys/self/export
```

查询参数：

- `start_timestamp`：开始时间 Unix 秒。
- `end_timestamp`：结束时间 Unix 秒。

响应包含：

- `generated_at`：生成时间。
- `start_timestamp`、`end_timestamp`：统计范围。
- `keys`：Key 汇总行。
- `models`：Key × 模型聚合行。

Key 汇总字段：Key ID、名称、脱敏 Key、状态、是否删除、请求数、输入 Token、输出 Token、总 Token、quota、使用模型数、最后调用时间。

模型明细字段：Key ID、Key 名称、脱敏 Key、模型、请求数、输入 Token、输出 Token、总 Token、quota、最后调用时间。

## Excel 工作簿

前端点击导出后请求聚合数据，并动态加载 `exceljs`，避免增加页面首屏包体。

工作簿包含三个工作表：

1. `数据概览`
   - 报告标题、账号、统计周期、生成时间。
   - 活跃 Key、使用模型、总请求、输入 Token、输出 Token、总 Token、消费金额。
   - 统计口径和隐私说明。
2. `Key 汇总`
   - 每个 Key 一行，默认按消费金额降序。
   - 包含状态、请求数、各类 Token、消费金额、消费占比、使用模型数和最后调用时间。
3. `模型明细`
   - 每个 Key × 模型一行，先按 Key，再按消费金额降序。
   - 包含请求数、各类 Token、消费金额、Key 内占比和最后调用时间。

视觉规范：

- 深蓝灰标题区、青绿色强调色，与新 UI 的清爽专业风格一致。
- Arial 字体、冻结表头、自动筛选、斑马纹、细边框和合理列宽。
- 金额、整数、百分比和日期使用原生 Excel 数字格式。
- 零值显示为 `-`，删除或禁用状态使用明确颜色。
- 文件名为 `Key消耗报告_<用户名>_<开始日期>_<结束日期>.xlsx`，非法文件名字符会被替换。

## 错误处理

- 查询失败时显示本地化错误提示，不生成文件。
- 生成过程中按钮显示加载状态并禁止重复点击。
- 无消费数据时仍生成完整报告，保留账号、时间范围和全部 Key。
- 动态加载 Excel 库或浏览器下载失败时统一提示用户重试。

## 验证

- 模型测试覆盖当前用户隔离、Key × 模型聚合、输入/输出 Token、零消费 Key、已删除 Key和时间边界。
- 控制器测试覆盖认证用户范围、非法时间和超过 30 天。
- 前端纯函数测试覆盖工作簿数据映射、排序、占比、文件名和零值。
- 执行受影响 Go 测试、Vitest、TypeScript 类型检查、目标文件 lint、生产构建与 `git diff --check`。
- 部署时只更新 `148.113.178.75` 的 master 节点，不操作 worker、CPA 或 Nginx，并验证容器健康与 `/api/status`。

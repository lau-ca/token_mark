# 模型能力配置持久化设计

> 此方案已被 `2026-08-11-model-capabilities-separation-design.md` 取代。模型能力现改为独立表存储，不再复用 `models.endpoints`。

## 目标

模型能力配置一旦保存，必须持续保存在 `models.endpoints` 中。渠道的新增、修改、启用、禁用和删除只能改变模型当前是否可用以及是否出现在 Playground，不能删除、覆盖或重建模型能力配置。

## 现状问题

`GET /api/models/capabilities` 当前以 `model.GetPricing()` 为模型清单来源。Pricing 又由启用的渠道能力生成，因此渠道状态或模型列表变化会改变能力配置页面的数据集合。持久化配置仍在 `models.endpoints` 中，但管理页面可能隐藏模型或把规则匹配模型显示为未配置。

Playground 使用当前启用渠道的模型列表，这是正确的；但能力配置保存和渠道修改后的 React Query 缓存没有完整联动，可能继续显示旧能力或旧可用性。

## 方案比较

### 方案 A：仅补前端缓存刷新

实现最小，但能力目录仍由渠道运行状态驱动。关闭唯一渠道后模型仍会从能力配置页面消失，不能满足持久化配置始终可管理的要求。

### 方案 B：能力目录改为持久化配置与运行时模型的并集

以 `models` 表中的精确模型配置为稳定来源，再合并当前启用渠道发现的模型。接口分别返回持久化配置和运行时可用性。无需新增数据库表，兼容现有 `models.endpoints`，改动集中且符合当前架构。

### 方案 C：新增独立模型能力表

边界最彻底，但需要跨 SQLite、MySQL、PostgreSQL 的迁移、回填和双写，当前能力 JSON 已有稳定存储位置，收益不足以覆盖复杂度。

采用方案 B。

## 后端设计

`GET /api/models/capabilities` 返回以下两类模型的并集：

1. `models` 表中 `name_rule = exact` 的模型记录，不受渠道状态影响。
2. 当前 Pricing 中存在、但还没有精确模型记录的运行时模型。

每项继续返回 `model_name`、`supported_endpoint_types` 和 `metadata`，并新增 `available`：

- `metadata` 始终来自精确模型记录，包含持久化的 `endpoints`。
- `supported_endpoint_types` 是当前渠道推断端点与持久化端点的合并结果。
- `available` 仅表示当前是否存在启用渠道，不参与配置保存。
- 仅存在于 `models` 表、没有启用渠道的模型仍返回，`available=false`。

接口不把推断端点写回 `models.endpoints`。渠道相关代码继续只维护 `channels`、`abilities` 和运行时缓存。

能力保存继续复用现有模型接口，但前端只基于接口返回的最新精确模型记录编辑 `endpoints`。本次不新增表、不迁移数据、不改变现有 JSON 结构。

## 前端设计

`/models/capabilities` 始终展示能力目录中的持久化模型。没有可用渠道时保留能力标签与编辑入口，并显示“当前无可用渠道”状态。

`/playground` 继续只展示当前分组下存在启用渠道的模型。渠道恢复后，Playground 从原有 `models.endpoints` 读取能力、参数和集成模板，不创建新配置。

渠道修改成功后统一失效以下查询：

- 模型能力目录。
- Playground 当前分组模型查询。
- Playground 全分组模型目录。

能力保存成功后同样失效 Playground 模型查询，避免五分钟缓存继续使用旧配置。

## 规则模型

能力配置页面只把精确模型记录作为可编辑持久化配置。规则记录继续用于运行时元数据匹配，但不会冒充具体模型的已保存配置。管理员为规则匹配出的具体模型保存能力时，仍创建对应的精确模型记录。

## 测试

- 后端测试证明禁用或移除渠道后，能力目录仍返回精确模型及原始 `endpoints`，同时 `available=false`。
- 后端测试证明运行时模型与持久化模型取并集且不重复。
- 前端测试证明不可用模型仍显示已保存能力和不可用状态。
- 前端测试证明渠道修改和能力保存会失效能力目录及 Playground 查询。
- 执行受影响 Go 测试、Vitest、TypeScript 类型检查、涉及文件 lint、前端生产构建和 `git diff --check`。

## 部署

构建 `linux/amd64` 镜像，生成可校验的离线镜像包。部署前只读核对 `148.113.178.75` 的 master compose、镜像名、容器状态和健康检查；备份当前 master 镜像后加载新镜像，仅重建 `/root/gateway/master/docker-compose.yml` 中的 `new-api-master`，不更新 worker。部署后验证容器健康、本机 `/api/status` 和公网状态接口。

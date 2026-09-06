# 模型能力配置独立存储设计

## 目标

模型能力配置不再创建或修改模型元信息记录。`/models/metadata` 只展示 `models` 表中的模型元信息，`/models/capabilities` 使用独立的能力配置表。

## 数据结构

新增 `model_capabilities` 表：

- `id`：主键。
- `model_name`：模型名称，唯一索引。
- `config`：`TEXT`，保存完整 JSON，允许后续扩展未知字段。
- `created_time`、`updated_time`：Unix 时间戳。

`config` 使用以下顶层结构：

```json
{
  "endpoints": {
    "image-generation": {
      "capabilities": ["image.generate"],
      "parameters": [],
      "integration": {}
    }
  }
}
```

## 数据流

- 能力目录由当前可用模型与 `model_capabilities` 中已配置模型取并集。
- 能力保存通过独立接口按 `model_name` 整体更新 `config`，不调用模型元信息 CRUD。
- Playground 模型详情和请求校验从能力表读取配置。
- `models.endpoints` 继续保存端点的 `path`、`method` 等元信息，不再保存 `playground`。

## 迁移

数据库建表后执行幂等迁移：从每条 `models.endpoints` 的端点对象中提取 `playground`，写入能力表对应端点；写入成功后移除原 JSON 中的 `playground`。已有能力表配置优先，迁移只补充缺失端点。

迁移后仅删除可严格识别的旧能力占位记录：精确模型、启用状态和官方同步状态均为旧流程默认值，且描述、图标、标签、供应商及清理后的端点全部为空。有任何真实元信息或非默认状态的记录都继续保留。

## 验证

- 保存运行时模型能力不会新增 `models` 记录。
- 元信息端点迁移后保留 `path`、`method` 和未知字段。
- Playground 列表与请求校验读取独立能力配置。
- 迁移可重复执行且不会覆盖已有能力配置。

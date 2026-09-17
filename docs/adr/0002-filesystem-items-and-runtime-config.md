# ADR 0002：文件系统条目存储与运行时配置

## Status

Accepted

## Context

SusShare 不引入数据库，本地文件系统必须同时承担条目内容、元数据、TTL 清理和容量控制。Web UI 允许修改保留策略和登录密码，这些修改需要重启后仍然生效。

## Decision

### Item Storage

每个 Shared Item 使用一个最终目录：

```text
/data/items/<item-id>/
  meta.json
  content
```

`item-id` 由服务端生成，使用 `crypto/rand` 生成的 128 位随机值并以 URL-safe 十六进制编码。列表排序依赖 `created_at`，不依赖 ID 的时间顺序。

创建条目时先写入 `/data/.staging/<item-id>/`，完成内容和元数据写入、大小校验和必要的 fsync 后，原子重命名到最终目录。列表只读取 `/data/items/` 下的完整条目；启动时清理残留 staging 目录。

`meta.json` 使用严格 schema 的 JSON，至少包含：

```json
{
  "id": "string",
  "kind": "text | file",
  "title": "string",
  "filename": "string, file only",
  "content_type": "string",
  "size": 0,
  "created_at": "RFC3339",
  "expires_at": "RFC3339"
}
```

文本条目的 `content` 保存原始 UTF-8 字节，不包装成 `.txt` 文件。文件条目以流式方式写入，不得为了校验大小而完整读入内存。

### Expiry and Cleanup

保留策略是服务级默认值，条目创建时计算并固化 `expires_at`。修改策略只影响之后创建的条目。

读取列表时懒过滤已过期条目；TTL Worker 周期性删除过期条目目录。删除操作以条目目录为单位，不维护单独的墓表。

### Runtime Config

配置加载后形成内存快照。运行时修改只允许两个入口：

- 修改保留策略。
- 修改登录密码哈希。

Config Manager 使用互斥锁串行化读取快照、校验、原子写回配置文件和替换内存快照。写回使用同目录临时文件加 `rename`，失败时保留旧文件和旧内存配置。程序会重写 `config.yaml`，不保证保留管理员手写注释。

服务器静态配置与运行时设置使用同一个 schema，但 Web API 只能更新允许的运行时字段。监听地址、存储路径、最大文件大小、总容量和文本上限仍由服务器管理员配置。

## Consequences

- 文件系统成为唯一事实来源，重启后无需数据库即可恢复条目。
- 半成品条目永远不会进入列表，崩溃后只会留下可清理的 staging 目录。
- 配置写入有清晰的失败语义，不会出现内存与磁盘各说各话的常见烂账。
- 管理员手写注释会在设置修改后丢失，这是明确接受的代价。
- 元数据损坏的条目不能凭空修复，启动时应隔离而不是猜语义。

## Alternatives Considered

- SQLite 存元数据、文件系统存内容：更可靠，但对当前单实例临时共享板增加了备份、迁移和运维面。
- 单个全局索引文件：并发更新和崩溃恢复比按条目目录更差。
- 每次读取配置文件：无法处理并发修改，也容易把半写的配置当真。

## Context

本设计面向绿地仓库，当前没有实现代码或既有规范。动机参见 `proposal.md - Why`；行为契约见四个增量 spec。已接受的架构决策见 `docs/adr/0001`、`docs/adr/0002` 和 `docs/adr/0003`。

## Goals / Non-Goals

**Goals:**

- 用一个 Go 进程同时服务嵌入的 Web UI 和 REST API。
- 让文件系统在无数据库时具备可恢复的条目原子性、TTL 语义和容量控制。
- 保证认证、密码修改、预览响应和配置写入的安全边界清晰。
- 保持部署面只有两个持久卷：`/config` 与 `/data`。

**Non-Goals:**

- 不做个人账号、条目所有权、审核、分享链接权限或回收站。
- 不做多实例一致性、分布式锁、数据库迁移或横向扩容。
- 不实现服务端 ZIP、Office、字幕或媒体转码解析。
- 不提供桌面客户端、CLI 产品、WebSocket 或自动发现。

## Decisions

### 1. Go 单体使用标准库 HTTP 服务

使用 Go `net/http` 提供路由、中间件、文件响应和 HTTP 测试能力。只引入必要依赖：

- `golang.org/x/crypto/argon2`：Argon2id 哈希与验证。
- `gopkg.in/yaml.v3`：配置解析与序列化。

考虑过 Echo、 Gin 或 Chi。它们能减少路由样板代码，但本项目的路由数量很小，标准库能避免框架中间件语义和依赖升级面。路由按 API 前缀手工分发即可，不做通用路由框架抽象。

### 2. Web UI 使用嵌入的静态单页应用

前端使用 Go `embed` 打包的 HTML、CSS 和原生 JavaScript，不引入 React、Vite 或 Node 构建链。界面状态保持为：

- login / board 两个主视图。
- text / file 两个 composer 模式。
- item list、settings modal、preview modal 的展示状态。

这符合“打开就用”的定位，也避免 API 与前端框架生命周期绑死。需要浏览器 API 的复制、拖拽和上传进度直接在模块化 JS 文件中实现。

### 3. API 使用一个条目资源集合

REST API 保持为：

```text
GET    /api/v1/items
POST   /api/v1/items
GET    /api/v1/items/:id
GET    /api/v1/items/:id/content
GET    /api/v1/items/:id/preview
DELETE /api/v1/items/:id

POST   /api/v1/auth/login
POST   /api/v1/auth/logout

GET    /api/v1/settings
PUT    /api/v1/settings
POST   /api/v1/settings/password
```

`POST /api/v1/items` 按 `Content-Type` 分派：`application/json` 发布文本，`multipart/form-data` 上传文件。文件分片顺序处理，每个成功分片独立 promote 为一个条目；任一分片失败时保留已成功的条目并在响应中给出每个文件的结果。这样不会为了“全部成功或全部失败”引入批次事务。

浏览器侧的文件上传请求带自定义 `X-SusShare-Request: 1` 头。服务默认不开 CORS，跨站页面无法带自定义头完成预检后的请求；再加上 `SameSite=Strict` Cookie，形成双层防线。普通表单提交、错误 Content-Type 和缺失请求头都被拒绝。

### 4. 文件系统条目模型

存储布局：

```text
/data/items/<item-id>/meta.json
/data/items/<item-id>/content
/data/.staging/<item-id>/
```

`item-id` 是 `crypto/rand` 生成的 128 位随机值，编码为小写十六进制。`meta.json` 是严格 schema 的 JSON，`content` 是原始字节。创建流程：

1. 创建 staging 目录。
2. 流式写入 content，边写边检查单条目大小。
3. 写入并 fsync `meta.json`。
4. fsync staging 目录。
5. 原子 rename 到 `/data/items/<item-id>/`。
6. fsync items 目录。

启动时扫描 `/data/items/` 恢复条目，清理 `/data/.staging/`。发现元数据损坏、ID 不匹配、重复 ID 或缺少 content 的最终目录时，移动到 `/data/quarantine/` 并记录错误，不用猜测修复。

### 5. 容量使用字节预留

上传前在互斥锁下读取当前已用字节数和进行中的预留，校验 `max_file_size` 与 `max_storage_size`，成功则登记预留；失败则拒绝。staging 写入成功并 promote 后，预留转为已用；失败或放弃时释放。

这避免两个坑：

- 把 5GB 文件完整读进内存再检查。
- 两个并发上传都看到“还有空间”，最后一起提交导致超额。

服务重启后预留全部消失，重新从最终条目计算已用空间。

### 6. TTL 采用懒过滤加后台 Worker

每次列表和读取都检查 `expires_at`，过期即按不存在处理。后台 Worker 按固定周期扫描并删除过期目录，同时回收容量。该策略让 API 语义不依赖 Worker 是否刚好运行，也让 Worker 延迟不会泄露过期内容。

### 7. 认证使用进程内 Session 表

登录成功后生成 256 位 `crypto/rand` token，进程内保存 token 到过期时间的映射，Cookie 只返回随机 token，不加密任何身份声明。密码验证使用 PHC 格式的 Argon2id 哈希和固定比较时间流程。Session 设置合理空闲和绝对过期时间，重启即全部丢失。

修改密码必须按顺序完成：

1. 验证当前密码和新密码策略。
2. 生成新 Argon2id 哈希。
3. Config Manager 原子写回并替换内存快照。
4. 清空 Session 表。

写回失败则不清空 Session，不产生“磁盘还是旧密码、内存已是新密码”的分裂状态。

### 8. 预览安全使用白名单和响应头

服务端结合扩展名、`http.DetectContentType` 的初始字节嗅探和显式白名单生成 `preview_kind`：

- `inline-image`
- `inline-audio`
- `inline-video`
- `inline-pdf`
- `text`
- `download`

SVG、HTML 和 XML 永远不进入 inline 白名单。预览响应设置 `X-Content-Type-Options: nosniff`、`Content-Security-Policy: sandbox; default-src 'none'`、`Referrer-Policy: no-referrer` 和明确的 `Content-Disposition`。文本只由 Web UI 作为源码或纯文本渲染。浏览器是否支持某种媒体编码由客户端能力决定，不支持时 UI 回退下载。

### 9. 配置管理使用不可变快照

启动时解析 YAML、校验路径、时长和大小，生成不可变配置快照。运行时更新只在 Config Manager 互斥锁内执行：

1. 基于当前快照构造新快照。
2. 校验字段。
3. 序列化 YAML。
4. 写同目录临时文件。
5. fsync 后原子 rename。
6. 替换内存快照。

写回失败保留旧文件和旧快照。成功后允许丢失注释和格式。静态字段只在启动配置中生效；Web API 的 settings DTO 不包含这些字段，未知字段直接拒绝。

## Risks / Trade-offs

- [进程重启导致所有 Session 失效] -> 这是接受的 MVP 行为，避免持久 Session 存储和密钥轮换复杂度。
- [文件系统损坏无法自动修复] -> 启动时隔离损坏目录并保留数据等待人工处理，不猜测元数据。
- [浏览器媒体解码能力不一致] -> 预览失败时回退下载；服务端不做转码。
- [大文件上传占用 staging 磁盘] -> 通过单文件和总容量预留限制，并在失败后立即清理 staging。
- [没有分页] -> 有限 TTL、总容量上限和倒序列表先满足 MVP；若小条目数量成为真实瓶颈，再增加基于创建时间的分页。
- [配置注释会丢失] -> 用户已明确接受；文档和容器示例必须写清楚。
- [内存 Session 与配置写回存在顺序依赖] -> 密码修改先持久化再清空 Session，失败路径保留旧会话。

## Migration Plan

这是绿地部署，没有既有用户数据需要迁移。发布流程：

1. 准备 `/config/config.yaml`，包含有效 Argon2id 哈希、监听设置、存储路径和限制。
2. 挂载 `/config` 与 `/data` 启动单容器。
3. 验证登录、发布文本、上传文件、下载、安全预览、删除和 TTL。
4. 回滚时停止并替换容器；已保留的 `/config` 与 `/data` 不由回滚自动修改。

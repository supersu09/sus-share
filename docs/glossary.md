# SusShare Glossary

## Shared Item

共享板中的唯一展示和操作单元，分为文本条目和文件条目。公共字段至少包括不可变 ID、类型、标题、创建时间和过期时间。列表按创建时间倒序展示。

## Text Item

内容为 UTF-8 文本的 Shared Item。文本直接存储为条目内容，不为了共享而包装成 `.txt` 文件。默认大小上限为 1 MiB，由服务器配置调整。

## File Item

内容为一个二进制文件的 Shared Item。一次多文件上传创建多个 File Item；MVP 不引入上传批次实体。

## Retention Policy

服务级默认保留策略。MVP 只允许有限时长，范围为 1 小时到 7 天。发布新条目时，服务读取当前策略并计算该条目的 `expires_at`。之后修改策略只影响未来条目，不重算已有条目。

## Expiry

条目的绝对过期时间，创建时固化。读取列表时懒过滤已过期条目，后台 TTL Worker 定期清理磁盘数据。

## Storage Root

服务器配置的本地目录，例如 `/data`。它保存条目内容和元数据，是重启后恢复状态的唯一来源之一。

## Item Metadata

描述 Shared Item 的持久化数据。MVP 可以使用每个条目一个元数据文件，但元数据必须与内容放在同一个条目目录中，通过临时目录加原子重命名创建。

## Native Preview

由浏览器原生能力渲染的安全预览。SusShare 不实现压缩包、Office 或媒体格式解析器；未知类型、不一致类型和活跃文档走下载。

## Download

以附件响应返回条目原始内容的兜底路径。浏览器无法安全预览的内容由用户下载后在本地应用中打开。

## Authentication Password

访问整个 Web UI 和 REST API 的共享密码。它不识别个人用户，也不表示所有权；通过认证的请求拥有相同权限。

## Session

登录成功后签发给浏览器的会话 Cookie。Session 只解决“已输入共享密码”这一件事，不携带用户身份或条目所有权。修改密码或服务重启后，全部 Session 失效。

## Server Config

只能由服务器管理员修改的静态边界，例如监听地址、存储路径、HTTPS、最大文件大小、总容量和日志级别。

## Runtime Settings

Web UI 允许修改的运行时设置。MVP 只包含保留策略和登录密码，修改后立即生效并持久化。

配置重写不保证保留管理员手写注释。监听地址、存储路径、大小上限和文本上限仍属于 Server Config。

## Manual Refresh

用户主动触发 `GET /api/v1/items` 的行为。客户端不建立 WebSocket，不轮询，不订阅服务端事件。

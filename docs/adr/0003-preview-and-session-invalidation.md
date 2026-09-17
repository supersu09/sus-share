# ADR 0003：浏览器预览边界与 Session 失效

## Status

Accepted

## Context

文件预览不能只看“浏览器是否能渲染”，因为 SusShare 的内容来自输入共享密码后的任意上传。如果 HTML 或 SVG 在同源响应中直接执行，一段脚本就能调用同源 API 读取、发布或删除条目。登录密码修改也必须避免旧 Session 继续存活。

## Decision

### Preview

预览优先使用浏览器原生能力，不实现 ZIP、Office 或媒体解析器。服务端只维护安全内联白名单：

- 图片：除 SVG 外的常见浏览器原生图片格式。
- 音频和视频：浏览器原生支持的类型。
- PDF：浏览器原生 PDF 查看器。
- 文本：由 Web UI 作为源码或纯文本渲染，不由浏览器作为 HTML 执行。

HTML、SVG、XML 和未知或扩展名/MIME 不一致的类型不做同源内联渲染。能安全展示源的按文本源码显示；否则仅提供下载。

预览响应必须设置：

- `Content-Type`
- `X-Content-Type-Options: nosniff`
- `Content-Disposition: inline` 或 `attachment`
- `Content-Security-Policy: sandbox; default-src 'none'`
- `Referrer-Policy: no-referrer`

下载是兜底行为，不是失败路径。浏览器不支持的原生类型、未知类型和活跃文档都通过下载交给用户本地处理。

### Content Type

客户端上传的 MIME 不能直接信任。服务端结合文件扩展名和初始字节判断类型；两者与安全白名单不一致时按未知类型处理并强制下载。

### Session Invalidation

Session 保存在服务进程内存中，Cookie 使用 `HttpOnly` 和 `SameSite=Strict`，默认不开启跨站 CORS。

修改密码成功后：

1. 先原子写入新的 Argon2id 密码哈希并替换内存配置。
2. 清空全部 Session 存储。
3. 当前浏览器收到认证失败后重新登录。

服务重启同样清空全部 Session，这是接受的 MVP 行为。

## Consequences

- 用户不需要为 ZIP 或 Office 文件安装服务端解析器。
- 预览覆盖常见媒体和 PDF，同时不会把任意 HTML/SVG 变成同源脚本执行环境。
- 修改密码后所有设备重新登录，语义简单且没有旧凭证残留。
- Session 重启即失效，持久登录不属于 MVP。

## Alternatives Considered

- 所有浏览器可渲染类型直接内联：实现最少，但同源 XSS 会破坏认证边界。
- 服务端解析压缩包和 Office 文件：复杂度和攻击面远超共享板 MVP。
- 数据库或文件持久化 Session：为了“改密码后仍保持登录”引入不值得的状态。

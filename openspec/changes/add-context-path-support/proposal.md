## Why

SusShare 目前假定自己部署在站点根路径。放在 Nginx 的 `/share/` 子路径后面时，页面 HTML 仍会请求 `/style.css`、`/app.js` 和 `/api/...`，登录 cookie 也写入 `Path=/`，子路径部署不可用。

## What Changes

- 新增 `server.base_path` 配置，表示对外暴露的 URL 前缀；默认保持根路径部署不变。
- 配置为 `/share` 后，应用会生成和接受位于 `/share` 下的 Web 与 API URL，并在代理保留前缀时先剥离该前缀再路由。
- 登录与登出的会话 cookie 使用该外部前缀作为 `Path`。
- 静态资源引用与前端 API 调用改为相对路径，避免把前缀复制到每个模板和 JS 字符串中。
- 更新配置和 Nginx 反向代理部署说明，并提供 `/share/` 的示例配置。

## Capabilities

### New Capabilities

- `http-context-path`: 反向代理子路径部署下的 URL 前缀解析、URL 生成、静态资源访问与会话 cookie 作用域。

### Modified Capabilities



## Impact

- 配置：`internal/config` 新增并校验 `server.base_path`；默认值保持当前行为。
- 路由：`cmd/sus-share/main.go` 需要处理外部前缀与内部路由的边界。
- Web：`internal/web/static` 改用相对资源与 API 路径。
- Auth/API：会话 cookie 的 `Path` 需要跟随外部前缀。
- 文档：README 和 `config.example.yaml` 需要说明配置与 Nginx 示例。
- 兼容性：`server.base_path` 缺省为空时，现有根路径部署和 cookie 行为不变；变更前缀后旧 cookie 不再发送到新前缀，用户需要重新登录。

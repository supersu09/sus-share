## Why

当前发送区高度跟随内容与浏览器 resize 行为，用户无法稳定获得足够的输入空间；设置和预览对话框依赖浏览器默认 dialog 位置，视觉上不够一致。需要把发送区变成明确可调的固定布局，并统一模态框与上传区域的视觉对齐。

## What Changes

- 发送区在进入看板时默认高度为 200px，仍固定在视口底部。
- 发送区顶部提供拖拽把手，用户可向上或向下拖动改变发送区高度；高度调整时条目列表可用空间同步变化，不产生遮挡。
- 移除文本框自身的浏览器 resize 行为，避免与发送区拖拽互相打架。
- 设置、查看、预览等 `<dialog>` 模态框在可用视口中水平和垂直居中，内容超高时在模态框内部滚动。
- 文件上传区域（拖拽/选择区）与文本框使用相同宽度，即同一 composer 内容宽度，不因表单切换而改变。
- 不修改 REST API、认证、上传语义或预览安全分类。

## Capabilities

### New Capabilities

### Modified Capabilities

- `web-ui`: 修改固定底部发送区的尺寸与调整行为，并约束模态框居中及上传区域与文本框同宽的布局要求。

## Impact

- `internal/web/static/index.html`
- `internal/web/tailwind/input.css`
- `internal/web/static/style.css`（由 Tailwind 构建生成）
- `internal/web/static/app.js`（如需拖拽状态与指针事件处理）
- 现有 Web UI 布局检查：桌面、移动、窄高视口、文本/文件模式切换、设置与预览模态框。

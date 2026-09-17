## Context

Web UI 是嵌入 Go 服务的原生单页应用：`index.html` 提供结构，`app.js` 处理交互，`tailwind/input.css` 生成 `static/style.css`。看板使用 `100dvh` 布局，头部固定，条目列表独立滚动，底部 composer 是 `flex` 剩余布局中的 shrink 子节点。设置和预览使用原生 `<dialog>`。构建脚本为 `pnpm build:css`。

## Goals / Non-Goals

**Goals:**

- 保持现有单文件前端架构，用一个明确的 composer 高度状态驱动布局。
- 让拖拽、键盘调整、模式切换和视口变化共享同一个边界计算。
- 让所有原生模态框在可用视口内居中，并在内容超高时内部滚动。
- 保持现有上传、发布、设置和预览行为不变。

**Non-Goals:**

- 不把 composer 高度保存到 localStorage 或服务端。
- 不新增前端框架、拖拽库或样式系统。
- 不改变 REST API、认证、文件上传和预览安全规则。
- 不为所有区块引入新的通用 resize 机制。

## Decisions

### 1. Composer 使用固定外层高度，而不是拉伸文本框

给发送区外层 `<section>` 建立默认 `height: 200px`，内部使用 `min-height: 0` 的纵向 flex/grid：模式切换行固定，当前表单占剩余空间，提交按钮固定。文本框改为占满表单剩余高度并禁用原生 `resize`。

备选方案是继续让 textarea 自己 resize，再把表单其他部分挤出可视区。这已经有重叠风险，而且文本模式和文件模式无法共享同一个高度模型，放弃。

边界用统一常量处理：默认 `200px`，下限保留完整操作行和可读输入区，上限保留头部与至少一小块可滚动的条目列表区域；窗口尺寸变化时重新夹紧。具体像素下限在实现中以 200px 高度下文本、文件两种模式均无重叠为准。

### 2. 拖拽把手使用 Pointer Events

在 composer 顶边放一个可聚焦的 separator 控件，设置上下 resize 光标和 `touch-action: none`。`pointerdown` 后捕获指针，`pointermove` 根据纵向位移更新高度，`pointerup` 或 `pointercancel` 结束。键盘 ArrowUp / ArrowDown 按固定步长调整，并同步 separator 的 ARIA 值。

备选方案是监听全局 `mousemove` 或引入拖拽库。前者要手工清理状态且触屏不稳，后者为一条垂直线引入依赖，都是多余复杂度，放弃。

### 3. 文本与文件模式共享 composer 内容盒

两个表单放在同一个 `max-w-3xl` 内容容器中，表单和上传 drop zone 都使用 `width: 100%` 与 `min-height: 0`。文件名列表在矮高度下滚动，提交按钮保持可见。

备选方案是只把 textarea 设为 `w-full`，上传区域继续使用自身最小高度。那会让模式切换时视觉宽度或内容盒高度不一致，放弃。

### 4. 模态框继续使用原生 dialog 布局

为 `.modal:modal` 设置 `margin: auto`，宽度保持现有响应式上限，最大高度限制为 `calc(100dvh - 2rem)`，并让超高内容在 dialog 内滚动。保留现有 backdrop 与关闭行为。

备选方案是改成 fixed overlay 加自研居中容器。那会重写焦点管理和层叠行为，收益为零，放弃。

## Risks / Trade-offs

- [200px 在极矮视口可能挤压表单] -> 用上下限和 `min-height: 0` 布局夹紧，并在短视口浏览器检查中验证无重叠；必要时视口自身滚动约束由应用级 `100dvh` 布局兜底。
- [拖拽与页面文本选择互相干扰] -> drag 控件禁用文本选择并捕获指针，拖拽期间阻止默认行为。
- [文件较多时占据提交按钮空间] -> 文件列表在 composer 表单剩余空间内滚动，提交按钮保持固定。
- [dialog 整体滚动可能让标题滚出视口] -> 保持模态框最大高度并优先让 body 区域滚动；实现时检查长文本、PDF 和设置对话框。
- [生成 CSS 与手写 CSS 漂移] -> 所有源样式只修改 `tailwind/input.css`，实现后必须运行 `pnpm build:css` 再检查 `static/style.css`。

## Migration Plan

1. 修改 HTML 结构、Tailwind 源样式和前端交互脚本。
2. 运行 `pnpm build:css` 生成静态样式。
3. 运行 Go 测试并用浏览器检查桌面、移动和窄高视口。
4. 回滚时恢复本次涉及的四个静态资源文件即可；无数据迁移和服务端配置变更。

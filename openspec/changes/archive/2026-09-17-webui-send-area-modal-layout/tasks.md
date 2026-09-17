## 1. 静态布局

- [x] 1.1 调整 `internal/web/static/index.html` 的发送区结构：添加可聚焦的顶部 resize separator、固定高度容器、`min-height: 0` 的文本/文件表单结构，并移除 textarea 原生 resize 类；检查 DOM 中拖拽控件、两种表单和上传区域的宽度容器关系正确。
- [x] 1.2 在 `internal/web/tailwind/input.css` 中实现 200px 默认高度、composer 内部 flex/grid、拖拽控件样式、文件列表内部滚动和高度上下限；运行 `pnpm build:css`，确认生成后的 `internal/web/static/style.css` 包含对应样式且构建成功。
- [x] 1.3 在 Tailwind 源样式中为设置与预览 dialog 添加居中、视口内最大高度和内部滚动约束；重新运行 `pnpm build:css`，检查生成样式没有直接手写漂移。

## 2. 交互行为

- [x] 2.1 在 `internal/web/static/app.js` 中实现基于 Pointer Events 的上下拖拽、统一高度夹紧、窗口变化重算、ArrowUp / ArrowDown 键盘调整和 separator ARIA 值同步；在浏览器中验证上拖变高、下拖变矮、指针移出控件仍连续拖动、释放后状态干净。
- [x] 2.2 验证键盘与重载行为：聚焦 resize separator 后用方向键调整高度，刷新页面后回到 200px；检查 ARIA 最小值、最大值和当前值随视口与高度更新。

## 3. 集成验证

- [x] 3.1 用真实浏览器在桌面、移动和窄高视口验证条目列表独立滚动、composer 无重叠、文本/文件模式同宽、文件名列表不挤掉提交按钮；保存或记录检查结果。
- [x] 3.2 验证设置、长文本查看、图片/视频/PDF 预览模态框均水平和垂直居中，超高内容只在模态框内部滚动且关闭行为不变；保存或记录检查结果。
- [x] 3.3 运行 `pnpm build:css` 和 `go test ./...`，确认静态资源可构建且既有 Go 测试全部通过；如环境缺少依赖，记录具体失败原因和已完成的浏览器验证。

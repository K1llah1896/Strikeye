# UI 无头渲染冒烟测试

## 为什么

本轮有大量 UI 改动（GSI 横幅、下载面板、分享码输入、SQL 控制台、音轨编辑器、区域控制、队伍视角、语音时间轴、头像刷新），但测试只覆盖纯函数——一个漏掉的 nil 判断或字段拼写错误只能在真机上点开才暴露。mygo 提供 `ui.Render(view, w, h, scale) *image.RGBA`，可以把整页渲染进图片，等于免费拿到"打开每个页面"的回归测试。

## 变什么

1. 新增 `internal/ui/renderutil`：`Probe` / `AssertRendered`（1280×800、4px 采样、统计不同颜色数，颜色过少判为空白）。
2. 五个视图各加渲染测试：
   - library：默认页、Steam 下载 + API Key 面板、GSI 直播横幅；
   - settings：全部卡片、带结果的 SQL 控制台；
   - analysis：八个标签页逐一渲染、展开选手抽屉（档案编辑器/趋势/购买行）；
   - highlights：合辑页与音轨编辑器、导出卡片；
   - spectator：选中比赛后的完整页面。
3. 数据用临时 SQLite + `UpsertByContentHash` 播种，不依赖真实 demo。

## 影响

- capability：`native-ui-shell-and-guard`（ADDED「Headless render smoke tests」）。
- 测试耗时约 1.2 秒，可在 CI 常规运行。

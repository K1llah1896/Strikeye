# Proposal: 选手跨场走势（抽屉内 sparkline）

## Why

`trends --steamid` 已经把选手逐场 Rating/ADR 拿出来，但只能在终端看。分析页的选手抽屉展示了多杀、TTK/TTD、道具 ADR 等当场指标，却没有任何"这个人最近几场是上升还是下滑"的信息——而这正是球探在档案里最先看的一行。

不需要图表库：仓库里已经有 `sparkline` 文本渲染（区域控制卡片在用），把逐场 Rating/ADR 画成 ▁▂▃▄▅▆▇█ 足够表达趋势，且不引入依赖、不增加 GPU 负担。

## What Changes

- 分析页选手抽屉新增"近 N 场"行：Rating sparkline + 首尾差值 + 胜场数、ADR sparkline + 首尾差值、最近三场对手与 Rating 摘要。
- 数据按需加载：展开选手行时后台查询一次并缓存在 `View.trends`，同一选手不重复查询；仅一场比赛时不显示该行。
- 渲染纯文本，不改动布局结构。

## Impact

- Affected specs: `tendency-report`（新增 requirement）
- Affected code: `internal/ui/views/analysis/analysis_trend.go`、`analysis.go`

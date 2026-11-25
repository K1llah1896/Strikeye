# Change: Kill Context (distance, line of sight, smoke)

## Why

击杀表现在只有"谁杀谁、什么枪、爆头、是否穿墙"，这些字段来自 demoinfocs 的事件标志。报告 4.4 的最后一条待办是"逐 kill 的 `kill_context` 明细"：把每一次击杀放回几何里，回答"这一枪隔了多远、中间有没有墙、有没有烟"。

数据已经齐了：`ReplayDeathEvent` 有双方坐标，`ReplayRoundData.Smokes` 有烟雾时间窗与中心，`cache/map_geometry/*.tri` 有碰撞几何与 BVH（TTD 已在用）。

## What Changes

- 新增 `internal/killctx`：给定回放回合、回合表、选手表与可选 BVH，输出每次击杀的 `distance`、`visible`（击杀者眼位到受害者胸口是否被几何遮挡）、`smoke_blocked`（连线是否穿过生效烟雾球），并汇总平均交火距离、被遮挡比例、穿烟比例。
- CLI `killctx [--demo-id]` 输出 JSON；无几何数据时仍给出距离与烟雾判定，`geometry_checked=false`。
- 区域控制页的击杀区位表增加「距离」「视线」两列：可见（绿）/ 遮挡（红）/ 遮挡·烟（黄），没有几何数据时显示"未知"。

## Impact

- Affected specs: `kill-context`（新增）、`match-library-and-analysis-ui`（修改击杀区位表）
- Affected code: `internal/killctx`、`cmd/strikeye-cli`、`internal/ui/views/analysis`
- 依赖：地图几何（`maps-fetch`）——缺失时降级而不是报错

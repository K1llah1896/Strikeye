# Proposal: 2D 回放的击杀信息条

## Why

报告 §4.2 的对标项里，"击杀 feed"是 CS:DM 2D viewer 的基础构件，我们只有地图上的死亡叉号——谁杀了谁、用的什么枪、是不是爆头，全要滚回回合表去查。回放播放时这条信息必须和画面同步。

## What Changes

- 画布新增 `ReplayLayers.KillFeed`（默认开）与 `SetSlotNames`/`SetTickRate`。
- `drawKillFeedLocked`：取最近 6 秒内最多 4 条击杀，按时间升序排成右上角面板（击杀者 ▸ 死者 + 武器名 + 爆头红点），越旧越淡。
- 武器名由击杀者最接近的弹道事件推断（`weapon_` 前缀剥除，±64 tick 内），避免为击杀事件新增字段。
- 分析页新增「击杀信息」复选框，选中 demo 时用 `spec_slot` 建立 slot→选手名映射；缺映射回退 `#<slot>`。

## Impact

- Affected specs: `replay-and-input-stream`
- Affected code: `internal/ui/components/radar_2d_canvas.go`、`internal/ui/views/analysis/analysis.go`

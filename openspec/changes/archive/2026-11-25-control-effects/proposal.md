# Change: Smoke- and Fire-Aware Control

## Why

控制曲线目前只按可达性算"谁先到"，把地图当成空地：一颗封住 A 大的烟、一道隔断 B 洞的火，在模型里完全不存在。报告 4.5 把这条列为控制曲线的最后一个已知偏差项——awpy 的 `map_control` 三模型里，烟雾挡视线、火焰挡路径，是标配。

回放缓存里已经有这些数据：`ReplayRoundData.Smokes` 带生效/失效 tick 与中心坐标，`ReplayRoundData.Infernos` 带火焰凸包与时间窗。缺的只是把它们接进控制判定。

## What Changes

- `mapcontrol.Config` 新增 `ApplyEffects`（默认开）与 `SmokeRadius`（默认 144 世界单位）。
- 采样时计算两份集合：
  - **烟雾遮蔽**：质心落在生效烟雾球内（XY 半径 144、Z 容差 144）的区域不计入归属；
  - **火焰封路**：质心落在生效火焰凸包内（2D 射线法）的区域既不计入归属，也不参与 Dijkstra 展开。
- `Frame` 新增 `blocked` 字段，记录该采样的遮挡区域数；净值仍按 (CT−T)/已分类 计算，遮挡区域不进入分子分母。
- CLI `mapcontrol` 新增 `--effects`（默认 true），可用 `--effects=false` 复现旧曲线做对照。
- 区域控制页的控制卡片增加"烟雾/火焰遮挡平均 N 个区域"，并在说明里点明遮挡区域不计入归属。

## Impact

- Affected specs: `map-control`（修改 Reachability control model）
- Affected code: `internal/mapcontrol`、`cmd/strikeye-cli`、`internal/ui/views/analysis`
- 性能：dust2 19 回合 409 采样仍 < 1 秒（每采样多 2242 次质心判定，实测 0.7 秒）。

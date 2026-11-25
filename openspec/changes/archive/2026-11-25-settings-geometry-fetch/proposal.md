# 设置页碰撞几何下载

## 为什么

TTD 与击杀视线判定（`internal/killctx`）依赖 `cache/map_geometry/<map>.tri`，但设置页的地图数据卡片只下载导航网格与 callout —— 用户按界面提示下载完，TTD 仍然是 0，只能再回终端跑 `maps-fetch`。

## 变什么

1. 地图数据卡片新增：
   - 状态行显示「碰撞几何 N 张」；
   - 「下载碰撞几何（全部官方图）」按钮（`mapgeom.FetchManifest` + `EnsureMaps`，写入 `<DataRoot>/cache/map_geometry`）；
   - 缺失提示改写为"导航网格 → 区域控制/地图控制，碰撞几何 → TTD/视线，callout → 地名"。

## 影响

- capability：`match-library-and-analysis-ui`（MODIFIED「Map data cache controls」）。
- 复用 `internal/mapgeom` 的校验和下载与 `.part`→rename，无新依赖。

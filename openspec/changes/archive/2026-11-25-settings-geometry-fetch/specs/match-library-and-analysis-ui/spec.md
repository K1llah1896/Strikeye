# match-library-and-analysis-ui

## ADDED Requirements

### Requirement: Geometry download in the map data card

设置页地图数据卡片 SHALL 覆盖碰撞几何：

- 卡片状态行 SHALL 显示已缓存碰撞几何数量（`<DataRoot>/cache/map_geometry` 下的 `.tri` 文件数）。
- 「下载碰撞几何（全部官方图）」SHALL 复用 `internal/mapgeom` 的清单校验与 `.part`→rename 下载，写入 `<DataRoot>/cache/map_geometry`。
- 下载 SHALL 在后台执行并复用卡片的进行中标志；失败时 SHALL 保留原有缓存不动并通过状态条回报。
- 任一数据缺失时卡片 SHALL 说明受影响的功能（导航网格 → 区域控制/地图控制，碰撞几何 → TTD 与击杀视线，callout → 地名）。

#### Scenario: First download

- **WHEN** 缓存为空时用户点击「下载导航网格（全部官方图）」
- **THEN** 状态条显示下载中，完成后显示"导航网格已就绪 N 张"，卡片计数同步更新

#### Scenario: Geometry download

- **WHEN** 用户点击「下载碰撞几何（全部官方图）」
- **THEN** `cache/map_geometry` 出现 `.tri` 文件，状态行几何计数增加，TTD 分析可算出结果

#### Scenario: Failed download

- **WHEN** 下载因网络错误失败
- **THEN** 状态条显示「下载失败: …」，已存在的缓存文件不被破坏

# match-library-and-analysis-ui

## ADDED Requirements

### Requirement: Map data cache controls

设置页 SHALL 提供地图数据下载入口：

- 卡片 SHALL 显示已缓存导航网格数量、callout 文件大小（缺失时显示「缺失」）与最近检查时间。
- 「下载 callout」与「下载导航网格（全部官方图）」SHALL 在后台执行，下载期间显示「下载中...」并阻止重复触发。
- 下载 SHALL 复用 `internal/mapdata` 的校验和与 `.part`→rename 逻辑。
- 成功与失败 SHALL 通过设置页状态条回报；失败时 SHALL 保留原有缓存不动。
- 缓存路径 SHALL 为 `<DataRoot>/cache/map_nav` 与 `<DataRoot>/cache/map_data/callouts.csv`。

#### Scenario: First download

- **WHEN** 缓存为空时用户点击「下载导航网格（全部官方图）」
- **THEN** 状态条显示下载中，完成后显示"导航网格已就绪 N 张"，卡片计数同步更新

#### Scenario: Failed download

- **WHEN** 下载因网络错误失败
- **THEN** 状态条显示「下载失败: …」，已存在的缓存文件不被破坏

#### Scenario: Callout missing

- **WHEN** `cache/map_data/callouts.csv` 不存在
- **THEN** 卡片显示「callout 缺失」并给出下载按钮

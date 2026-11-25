# 批量导出（多场一次落盘）

## 为什么

`export` 目前一次只能导一场（`--demo-id`）或一名选手的跨场聚合（`--steamid`）。分析师真正的工作方式是"把这个系列赛/这支队的全部地图导出来，批量丢进表格做透视"，现在只能写 shell 循环，而且每场文件都落在同一个目录里（`match_<id>_*` 靠 id 前缀区分），批量导出时还会和既有文件混在一起。调研报告 §4.7 的"可选：批处理与脚本化"就是这一条。

## 变什么

1. `strikeye-cli export --all` 批量导出所有已解析比赛，每场写到独立目录 `<out>/demo_<id>/`。
2. 过滤器：`--team`（队名，大小写不敏感，A/B 任一侧命中即可）、`--map`（地图名）、`--limit`（最多 N 场）。
3. 批末写 `<out>/export_index.json`：过滤条件、成功/失败计数、每场的队伍/地图/目录/文件列表/错误。
4. 单场失败不中断整批，只在索引里记 `error`；全批都失败才返回错误。
5. `--all` 与 `--demo-id`/`--steamid` 互斥。

## 影响

- capability：`match-data-export`（ADDED「Batch export across matches」）。
- 新增 `internal/export/batch.go`（`BatchFilter`/`BatchItem`/`BatchResult`/`Batch`）与 CLI 接线；单场导出行为不变。

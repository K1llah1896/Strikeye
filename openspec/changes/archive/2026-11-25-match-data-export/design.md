# Design: 比赛数据导出

## 结构

`internal/export` 提供纯函数式 API，不依赖 UI：

- `LoadMatchBundle(ctx, db, demoID) (MatchBundle, error)`：读取 demo、阵营、选手、回合、事件。
- `WriteMatchBundle(bundle, dir, Formats) ([]string, error)`：CSV 拆分为 `match_<id>_players.csv`、`_rounds.csv`、`_events.csv`、`_sides.csv`；JSON 合并为 `match_<id>.json`。
- `AggregatePlayer(ctx, db, steamID) (PlayerAggregate, error)`：遍历 demo 的 `match_players`，汇总 totals 并按比赛输出明细行。
- `WritePlayerAggregate(agg, dir, Formats) ([]string, error)`：`player_<steamid>_matches.csv` 与 `.json`。

## 列定义

CSV 表头固定且顺序稳定（便于 Excel/SQL 导入）；浮点保留两位小数。JSON 顶层字段使用 snake_case（`schema_version`、`demo`、`players`、`rounds`、`events`、`sides`），明细记录沿用存储层结构体字段名，包含 `schema_version: 1` 便于未来演进。

## 聚合口径

- 合计比率按"总量/总回合"重算，而不是对每场比率取平均：`ADR = Σdamage / Σrounds`，`KAST% / HS% / Rating` 同理采用加权平均（Rating 以回合数加权）。
- 只统计 `parse_status = ready` 的比赛；缺失的选手行跳过。
- 同一 SteamID 在不同比赛中的名字取最近一场。

## CLI

`strikeye-cli export --demo-id 12 --format both --out <dir>` 或 `--steamid 765...`。未指定 `--out` 时使用 `<DataRoot>/media/exports`。成功时逐行打印绝对路径。

## 兼容性

不改数据库 schema；导出为只读操作。

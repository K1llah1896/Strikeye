# kill-context

## ADDED Requirements

### Requirement: Stored kill context rows

击杀上下文 SHALL 可持久化并查询：

- 迁移 v12 SHALL 创建 `kill_context` 表，主键 `(demo_id, round_number, tick, victim_slot)`，并建立 `(demo_id, round_number)` 与 `(demo_id, smoke_blocked)` 索引。
- `killctx --store` SHALL 在单事务内重写该 demo 的全部行（空结果即清空）。
- 每行 SHALL 记录武器（按同槽位 ±64 tick 内最近一发推断）、爆头、交火距离、视线与几何/光学检查标记、穿烟、开镜夹角、是否在视锥内、受害者是否看到击杀者、双方致盲与开镜等级、双方世界坐标。
- `Totals` SHALL 在 SQL 中聚合击杀数、平均距离、背身率与穿烟率。
- 比赛导出 SHALL 包含击杀行：CSV `match_<id>_kills.csv`、XLSX `kills` 表、JSON `kills` 数组。

#### Scenario: Store then query

- **WHEN** 用户对一场 demo 运行 `killctx --store`，然后执行 `strikeye-cli sql --query "select count(*) from kill_context"`
- **THEN** 返回该场的击杀条数，且每行都有武器与坐标

#### Scenario: Export includes kills

- **WHEN** 已落库的 demo 导出 CSV
- **THEN** `match_<id>_kills.csv` 出现，行数与库内一致，`distance` 保留两位小数

#### Scenario: Re-store replaces

- **WHEN** 同一 demo 再跑一次 `--store`
- **THEN** 表中仍只有一份数据（不重复累积）

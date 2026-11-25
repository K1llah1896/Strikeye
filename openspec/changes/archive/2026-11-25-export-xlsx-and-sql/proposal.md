# Change: XLSX Export and Read-Only SQL Console

## Why

报告 4.7 把数据出口列为与 CS:DM 的差距：对方能导出 XLSX、能用 SQL 查库。当前只有 CSV / JSON / HTML，分析师拿到数据后还得自己拼表；想回答"某个选手在 CT 侧、经济局、长枪局的 ADR"这种组合问题，只能写代码。

## What Changes

- 新增 `internal/xlsx`：零依赖的最小 OOXML 写入器（内联字符串 + 数字单元格 + 加粗表头），支持多工作表。
- `export` 增加 `--format xlsx`：
  - 比赛包：`match_<id>.xlsx`，四张表 `players / rounds / events / sides`；
  - 选手聚合：`player_<steamid>_matches.xlsx`。
- 新增 `internal/sqlconsole` + CLI `sql --query <sql> [--format csv|json] [--limit n]`：
  - 只接受单条 `SELECT` / `WITH` / `EXPLAIN`；
  - 关键字黑名单拦截写操作、`ATTACH`、`PRAGMA`；
  - 连接层再开 `PRAGMA query_only(1)`，双保险；
  - 默认 200 行上限，超出时标记 `truncated`。

## Impact

- Affected specs: `match-data-export`（新增 XLSX 要求）、`sql-console`（新增）
- Affected code: `internal/xlsx`、`internal/export`、`internal/sqlconsole`、`internal/storage`（`ReadOnly`）、`cmd/strikeye-cli`
- 无新增第三方依赖

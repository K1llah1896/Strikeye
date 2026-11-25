# Spec Delta

## Purpose

给分析师一个受控的只读 SQL 入口，用来回答 UI 与固定导出没有覆盖的组合问题。

## ADDED Requirements

### Requirement: Read-only SQL console

系统 SHALL 提供 `strikeye-cli sql --query <sql>`，并满足：

- 只接受单条语句，且首关键字必须是 `SELECT`、`WITH` 或 `EXPLAIN`。
- 系统 SHALL 拒绝包含 `INSERT`、`UPDATE`、`DELETE`、`REPLACE`、`DROP`、`ALTER`、`CREATE`、`ATTACH`、`DETACH`、`PRAGMA`、`VACUUM`、`REINDEX`、`TRIGGER`、`BEGIN`、`COMMIT`、`ROLLBACK` 的查询。
- 校验前 SHALL 剥离字符串字面量与注释，避免字面量内容造成误判或漏判。
- 查询连接 SHALL 启用 `PRAGMA query_only(1)`，使引擎层拒绝任何写入。
- 输出 SHALL 支持 `json`（列名 + 行 + 是否截断）与 `csv` 两种格式。
- `--limit` SHALL 限制返回行数，默认 200，0 表示不限制；超出上限时结果标记 `truncated=true`。
- 查询出错 SHALL 返回非零退出码并打印引擎错误。

#### Scenario: Aggregate query

- **WHEN** 用户查询按地图统计的击杀数
- **THEN** 输出包含 `columns=["map_name","kills"]` 与每张地图一行

#### Scenario: Write attempts are rejected

- **WHEN** 用户执行 `DELETE FROM demos`
- **THEN** 命令以校验错误退出，数据库不变

#### Scenario: Statement chaining is rejected

- **WHEN** 用户执行 `SELECT 1; DELETE FROM demos`
- **THEN** 命令拒绝执行，因为包含多条语句

#### Scenario: Row cap marks truncation

- **WHEN** 查询命中 137 行而 `--limit` 为 1
- **THEN** 只返回 1 行且 `truncated` 为 true

#### Scenario: CSV output

- **WHEN** 用户以 `--format csv` 查询
- **THEN** 第一行是列名，后续每行是按列顺序格式化的值

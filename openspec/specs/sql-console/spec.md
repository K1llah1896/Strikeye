# sql-console Specification

## Purpose
给分析师一个受控的只读 SQL 入口，用来回答 UI 与固定导出没有覆盖的组合问题：自定义关联、临时过滤条件，以及与外部工具对接时的数据抽取。所有查询在引擎层被限制为只读，写入语句在校验与执行两层都会被拒绝。

## Requirements

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

### Requirement: Desktop console card

设置页 SHALL 提供只读 SQL 控制台：

- 卡片 SHALL 提供若干常用查询预设与自由输入框，预设可一键载入。
- 执行 SHALL 走 `sqlconsole.Validate` + `storage.DB.ReadOnly`（`query_only`）双重限制，写语句 SHALL 被拒绝并显示原因。
- 结果 SHALL 显示列名与前 20 行，并标注总行数；超过上限 SHALL 标记截断。
- 「导出 CSV」SHALL 把完整结果写到 `<DataRoot>/media/exports/query_<时间戳>.csv`。
- 查询 SHALL 有 200 行上限与 15 秒超时，并在后台执行。

#### Scenario: Run a preset

- **WHEN** 用户选择一个预设并点击「载入」再点「运行」
- **THEN** 结果表显示列名与行数据，状态行显示行数与列数

#### Scenario: Write statement rejected

- **WHEN** 用户输入 `delete from demos` 并运行
- **THEN** 卡片显示校验错误，数据库不被修改，按钮不再显示"查询中"

#### Scenario: Export the full result

- **WHEN** 用户点击「导出 CSV」
- **THEN** `media/exports` 下出现 `query_*.csv`，包含表头与全部行（不受 20 行显示限制）

# sql-console

## ADDED Requirements

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

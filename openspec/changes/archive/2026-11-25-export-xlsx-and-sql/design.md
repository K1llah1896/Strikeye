# Design: XLSX Export and Read-Only SQL Console

## 为什么自己写 XLSX

引入 excelize 会带来一个数万行的依赖，而我们只需要"多张表、表头加粗、数字保持数字"。最小 OOXML 由 7 个部件组成：`[Content_Types].xml`、`_rels/.rels`、`xl/workbook.xml`、`xl/_rels/workbook.xml.rels`、`xl/styles.xml`、每个工作表的 `xl/worksheets/sheetN.xml`。写入器约 200 行，测试可以直接解压检查 XML。

单元格策略：

- 数字 → `<c r="B2"><v>47</v></c>`，保持可计算；
- 其它 → 内联字符串（`t="inlineStr"`），避免 sharedStrings 表；
- 表头 → `s="1"` 引用加粗字体样式。

数值判定复用 CSV 行：`xlsxRows` 对每个字段尝试 `ParseFloat`，成功就写成数字。这样 CSV 与 XLSX 的列定义永远一致，不会出现"CSV 有 44 列、XLSX 有 43 列"的漂移。

## 为什么 SQL 控制台默认只读

数据库里全是可重解析的派生数据，但仍然不该让一条误粘贴的 `UPDATE` 毁掉整库。三层防护：

1. **语法层**：只允许 `SELECT` / `WITH` / `EXPLAIN`，且必须单语句；字符串字面量与注释先剥离，避免 `SELECT 'delete from demos'` 误判，也避免 `SELECT 1; DROP ...` 漏判。
2. **关键字层**：`insert/update/delete/replace/drop/alter/create/attach/detach/pragma/vacuum/reindex/trigger/begin/commit/rollback` 一律拒绝。
3. **引擎层**：独立连接开 `PRAGMA query_only(1)`，即使前两层被绕过，SQLite 自己也会拒绝写入。

行数上限通过 `SELECT * FROM (<query>) LIMIT n+1` 包装实现，多取一行用于判断是否截断。这样即使查询自带 `LIMIT` 也不会被改写成错误语义，代价是外层包一层子查询（SQLite 会把 LIMIT 下推）。

## 已知取舍

- 包装会改变 `EXPLAIN` 的输出（变成子查询计划），对分析用途可以接受。
- 不提供 `--format html`，SQL 结果表格交给 XLSX/CSV。
- `db.ReadOnly` 使用普通连接 + `query_only`，而不是 `mode=ro`：WAL 模式下的只读打开在缺少 `-shm` 时会失败，`query_only` 更稳。

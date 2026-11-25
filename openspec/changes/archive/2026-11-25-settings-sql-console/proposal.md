# 只读 SQL 控制台（GUI）

## 为什么

`strikeye-cli sql` 已经能安全地跑只读查询，但分析师在用桌面应用时想核对一个数字（"这张图上谁 ADR 最高"）仍要开终端。只读控制台是最便宜的"用户自己回答剩余问题"的工具，也是报告 §4.10 数据库能力的自然延伸。

## 变什么

1. 设置页新增「只读 SQL 控制台」卡片：
   - 4 个常用查询预设（选手榜单、地图场次、回合胜负、武器击杀）+ 自由输入；
   - 「运行」在后台执行，结果以等宽表格显示前 20 行，标注总行数与截断；
   - 「导出 CSV」把完整结果写到 `<DataRoot>/media/exports/query_<时间戳>.csv`。
2. 安全三层：`sqlconsole.Validate` 白名单 + `storage.DB.ReadOnly`（`_pragma=query_only(1)`）+ SQLite 自身只读连接。
3. 查询上限 200 行、超时 15 秒。

## 影响

- capability：`sql-console`（ADDED「Desktop console card」）。
- 复用 `internal/sqlconsole` 与既有 `ReadOnly` 连接，无新表、无新依赖。

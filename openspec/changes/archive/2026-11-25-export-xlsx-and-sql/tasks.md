# Tasks

## 1. XLSX writer
- [x] 1.1 `internal/xlsx`：多工作表、加粗表头、内联字符串、数字单元格
- [x] 1.2 工作表名清洗（31 字符、非法字符）
- [x] 1.3 列名 A1 转换
- [x] 1.4 单测：部件齐全、转义、数字、空工作簿、XML 合法

## 2. Export integration
- [x] 2.1 `Formats.XLSX` 与 `--format xlsx`
- [x] 2.2 比赛包四表工作簿
- [x] 2.3 选手聚合工作簿
- [x] 2.4 CSV/XLSX 共用列定义（表构建函数重构）

## 3. SQL console
- [x] 3.1 `internal/sqlconsole` 校验与执行
- [x] 3.2 `storage.DB.ReadOnly` 使用 `query_only`
- [x] 3.3 CLI `sql`，JSON/CSV 输出与行数上限
- [x] 3.4 单测：允许的读查询、写入拒绝、多语句拒绝、WITH、截断、坏 SQL

## 4. Verification
- [x] 4.1 真实数据：`export --format xlsx` 生成 4 表工作簿；`sql` 按地图统计击杀 215/137
- [x] 4.2 `go test ./...`、`go vet ./...` 通过

# Tasks

- [x] 迁移 v12 `kill_context` + 两个索引
- [x] `KillContextRepo`（Replace/List/Totals）
- [x] `killctx.Build` 增 `Weapon` 与 `KillerPos`/`VictimPos`；`--store`
- [x] 导出：`_kills.csv` + XLSX `kills` 表 + JSON `kills`
- [x] 测试：仓库往返/Totals/清空、导出 CSV 与 JSON、`killsTable` 舍入
- [x] 真实数据：dust2 137 行落库，SQL 聚合平均距离 760.9、穿烟 10、背身 19
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 报告 §4.4 与 §8 更新

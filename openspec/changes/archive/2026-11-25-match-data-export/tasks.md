## 1. 导出核心

- [x] 1.1 `internal/export` 提供 `LoadMatchBundle`、`WriteMatchBundle`（CSV 四表 + 合并 JSON）
- [x] 1.2 `AggregatePlayer` 按 SteamID64 跨比赛聚合，ADR/KAST/Rating 按回合加权，HS% 用总爆头/总击杀
- [x] 1.3 JSON 带 `schema_version: 1`，CSV 表头固定、浮点两位小数

## 2. CLI 与路径

- [x] 2.1 `strikeye-cli export --demo-id|--steamid --out --format csv|json|both` 互斥校验
- [x] 2.2 默认输出到 `<DataRoot>/media/exports`，打印写入文件清单
- [x] 2.3 未知 demo / 无比赛选手返回非零退出并给出错误

## 3. 验证

- [x] 3.1 单测覆盖 CSV/JSON 文件集合、行数、表头、聚合加权值与错误路径
- [x] 3.2 `go test ./...` 全绿
- [x] 3.3 `openspec validate match-data-export --strict` 通过

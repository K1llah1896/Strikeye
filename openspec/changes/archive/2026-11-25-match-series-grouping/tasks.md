## 1. 系列赛推导

- [x] 1.1 `internal/series.Group` 按规范化对阵 + 比赛日分组，跳过未解析/占位队名/单图组
- [x] 1.2 队伍 A/B 锚点排序，演示列交换时翻转比分并标记 `swapped`
- [x] 1.3 `ByDemoID` 与 `Label`（`TeamA 2-1 TeamB`）供 UI/CLI 复用

## 2. 展示与入口

- [x] 2.1 `strikeye-cli series [--min-maps N]` 输出 `{"count":N,"series":[...]}`，空列表退出 0
- [x] 2.2 比赛库新增“按系列赛分组”开关，组内卡片连续排列
- [x] 2.3 多图系列在首张卡片前渲染 `系列赛 · TeamA x-y TeamB · N 图 · 日期` 头部；单图不渲染

## 3. 验证

- [x] 3.1 单测覆盖交换比分、单图/占位队名/未解析跳过、跨日分隔、地图排序、按 ID 反查
- [x] 3.2 CLI 测试用临时 SQLite 种子数据验证分组输出
- [x] 3.3 真实双图 Demo 验证：`series` 输出 Falcons 0-2 FURIA
- [x] 3.4 `go test ./...` 全绿；`openspec validate match-series-grouping --strict` 通过

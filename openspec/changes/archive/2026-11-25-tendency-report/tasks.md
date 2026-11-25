# Tasks

## 1. Aggregation

- [x] 1.1 `internal/tendency`: `Report`/`SideProfile`/`EconomyStat`/`SiteStat`/`OpeningStat`、`Input{Team,Replays,Callouts,TopN}`
- [x] 1.2 `Build`: 过滤 parsed + 队名匹配、分图分侧战绩与手枪局、平均装备与每回合道具、经济类型桶、T 侧下包点、开场击杀 callout 与耗时
- [x] 1.3 `teamSides` 只认 `TeamAName`/`TeamBName`；缺 callout 时 `callouts_used=false`

## 2. CLI

- [x] 2.1 `cmd/strikeye-cli/tendency.go`: `runTendencies`（`--team --top --format --out`）、`writeTendencyCSV`
- [x] 2.2 `main.go` 注册子命令与 usage
- [x] 2.3 缺 callout 时 stderr 提示 `callouts-fetch`

## 3. Verification

- [x] 3.1 `internal/tendency/tendency_test.go`: 侧别/经济/下包/道具/开场耗时、空队名、未解析 demo 忽略
- [x] 3.2 `cmd/strikeye-cli/tendency_test.go`: CSV 拉平结构、缺 `--team` 报错
- [x] 3.3 真实数据：FURIA 2 图 49 回合，开场击杀 27 次平均 14.54 秒，dust2 T 侧 B 5 次 80% / A 8 次 62.5%

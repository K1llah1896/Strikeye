## Why

报告 §3 的「实时 GSI」一直是 ✘，§9 把它列为 P2「与本地工具定位冲突」。但 GSI 是 Valve 官方、只读的本地推送（HTTP POST，游戏主动发），不需要注入、不需要插件——和"零侵入本地分析"的定位并不冲突，缺的只是一个接收端。

## What Changes

- 新增 `internal/gsi`：GSI 载荷解析（字符串/数字混排的字段用 `flexInt`/`flexFloat` 容错）、`Store`（最新状态 + 订阅 + 回调）、`Handler`（POST + Bearer token 校验 + 1MB 上限）、`Config`（生成 Valve KeyValues 格式的 `gamestate_integration_strikeye.cfg`）。
- CLI `gsi --listen --token --config-out --latest --once --timeout`：写配置、起 HTTP 服务、每条载荷输出一行摘要（ndjson），并把最新状态原子写到 `cache/gsi_latest.json` 供 UI/脚本轮询。
- 摘要含地图、比分、回合、炸弹状态、剩余回合时间、存活人数（按边）。

## Impact

- 新增：`internal/gsi/gsi.go`、`cmd/strikeye-cli/gsi.go`。
- 修改：`main.go` 子命令与 usage。
- 不修改游戏、不读取游戏内存；token 可选，为空时不校验。

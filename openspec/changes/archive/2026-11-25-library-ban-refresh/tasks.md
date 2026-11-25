# Tasks

- [x] `steamAPIKeyPath` / `saveSteamAPIKey` / `loadSteamAPIKey`（0600 + env 优先）
- [x] `refreshBans`（去重选手 → `vac.Service.Lookup` → 汇总与封禁名单）
- [x] `runBanRefreshNow`（90s 超时 + 状态条 + 刷新资料库）
- [x] 工具栏「刷新封禁」+ 无 Key 时 `renderAPIKeyPanel`
- [x] 测试：Key 往返与权限、env 优先、无选手报错、缓存行离线判定
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 报告 §8 更新

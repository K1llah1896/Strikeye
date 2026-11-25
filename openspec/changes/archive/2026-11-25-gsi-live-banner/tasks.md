# Tasks

## 1. Mirror reader

- [x] 1.1 `internal/ui/views/library/live.go`: `liveStatePath`、`loadLiveState(dataRoot, now, maxAge)`、`liveFreshness = 15s`
- [x] 1.2 `aliveCounts` + `liveHeadline`（地图/回合/比分/阶段/炸弹/存活），`carried`/`dropped` 省略
- [x] 1.3 `refreshLiveState` 2 秒节流，读失败静默降级

## 2. Banner

- [x] 2.1 `library.go`：View 增 `live gsi.State`/`liveOK bool`/`liveChecked time.Time`
- [x] 2.2 `Render` 顶部渲染玻璃态状态条（绿灯 + 「GSI 直播」+ 文本 + 来源说明），仅在 `liveOK` 时出现

## 3. Verification

- [x] 3.1 `live_test.go`：新鲜/缺失/过期、存活计数忽略已死与观战、横幅文本、`carried` 省略、`itoa` 边界
- [x] 3.2 `go test ./internal/ui/views/library/`、`go build ./...` 通过

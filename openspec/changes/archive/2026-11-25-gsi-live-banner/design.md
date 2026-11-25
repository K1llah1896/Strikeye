# Design: 资料库 GSI 直播条

## Context

`strikeye-cli gsi` 以 ndjson 输出并把最新状态原子写入 `cache/gsi_latest.json`（`.part` → rename）。UI 只需要读这个文件，不需要再开一个 HTTP 接收端：进程边界清晰，且 UI 崩溃不影响接收端。

## Decisions

1. **以文件时间戳判新鲜度**，而不是载荷里的 `ReceivedAt`：镜像由 CLI 写入，文件 mtime 就是"最后一次成功解析"的时间；载荷时间会被测试或重放的旧数据伪造。
2. **15 秒窗口**：GSI 在比赛中默认每 0.5–5 秒推送一次，15 秒足以覆盖一次暂停或回合间隔，又不会把上一场比赛的状态留在屏幕上。
3. **2 秒节流**：`Render` 每帧调用 `refreshLiveState`，用 `liveChecked` 做节流，避免每帧 `Stat`+`ReadFile`。
4. **缺失/过期不显示**：空状态条会说"直播中"却没有任何数据，比不显示更糟；CLI 侧的错误已经由 `gsi` 命令自己输出。
5. **不引入新依赖**：复用 `internal/gsi` 的 `State` 与 `json.Unmarshal`，`itoa` 手写实现以避免在渲染路径上多一次 `strconv` 依赖（也便于单测）。

## Risks

- 用户在另一台机器跑 CLI：镜像路径按 `DataRoot` 解析，跨机器不共享属预期行为，文档已注明同步 `DataRoot` 即可。
- `carried`/`dropped` 炸弹状态在直播条上没有信息量（几乎全程如此），按设计从横幅里省略，只在 `planted`/`defusing`/`exploded` 时显示。

## Migration

无 schema 变更；查询失败或字段缺失时横幅自动隐藏。

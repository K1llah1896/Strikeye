# Tasks

## 1. Linkage
- [x] 1.1 release workflow 去掉 `-H windowsgui`（控制台子系统）
- [x] 1.2 README 的手工构建命令同步

## 2. Console handling
- [x] 2.1 `detachConsoleForDesktop`：`GetConsoleProcessList` 判独占，`ShowWindow(SW_HIDE)` + `FreeConsole`
- [x] 2.2 桌面路径在 `main` 首行调用；共享控制台不动
- [x] 2.3 保留 `attachConsoleForCLI`（GUI 子系统自建版兜底）
- [x] 2.4 非 Windows 为 no-op

## 3. Verification
- [x] 3.1 `file strikeye.exe` → `PE32+ executable (console) MS Windows`
- [x] 3.2 `go build ./...`、`go test ./cmd/strikeye/`
- [x] 3.3 交叉编译三平台
- [x] 3.4 发布包实测：单 exe + 三边车，zip 完整性校验通过

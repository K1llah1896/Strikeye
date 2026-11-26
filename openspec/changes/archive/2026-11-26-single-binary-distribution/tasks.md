# Tasks

## 1. Dispatch
- [x] 1.1 `internal/cli` 取代 `cmd/strikeye-cli`（`Run(args []string)` 入口，独立 FlagSet）
- [x] 1.2 `cmd/strikeye/dispatch.go`：`resolveMode` + `classifyProgram` 纯函数
- [x] 1.3 `runAgentEntry`：`agent` 选择子复用既有代理启动
- [x] 1.4 Windows GUI 子系统下附着父控制台（`console_windows.go` / `console_other.go`）
- [x] 1.5 开发 shim：`cmd/strikeye-cli/main.go` 仅转调 `cli.Run`

## 2. Update path
- [x] 2.1 `applyZipPackage` 在合并包到达时删除旧 `strikeye-agent`/`strikeye-cli`（含 `.old`）
- [x] 2.2 过渡/降级包（仍带旧名）不删任何文件

## 3. Packaging
- [x] 3.1 release workflow 只构建并打包 `strikeye.exe`
- [x] 3.2 打包仍分 `zip -9`（exe）与 `zip -0`（边车）并 `zip -T`

## 4. Verification
- [x] 4.1 `go test ./... -count=1` 全绿（新增 `dispatch_test.go`、updater 两个用例）
- [x] 4.2 Linux 实测：`strikeye cli doctor`、`cli navs`、`cli parse`、`cli killctx`、`cli export --all` 全部正常
- [x] 4.3 重命名副本实测：`strikeye-cli --version` / `strikeye-agent --version` / `agent --env-check`
- [x] 4.4 打包实测：exe 63.6 MB，zip 255 MB → 215 MB
- [x] 4.5 文档：README 与调研报告记录单文件分发

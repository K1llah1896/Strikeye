## Why

报告 §3 把「跨平台」记为 ✘，实际代码早已带平台分支（`internal/ipc/transport_other.go` 用 Unix socket、`internal/recorder/process_exec_other.go` 用分离进程、`updater` 按 GOOS 选目标），但没有任何地方把它说清楚：用户在 Linux/macOS 上装完只会发现"录制不能用"，不知道其它功能是好的。

## What Changes

- 新增 `strikeye-cli doctor` 的 `platform` 段：`os`、`arch`、`ipc_local_socket`（`named_pipe` / `unix_socket`）、`hlae_recording`、`notes`。
- 平台差异显式化：Windows 之外 `hlae_recording=false` 并给出"分析/回放/导出/CLI 可用"的说明。
- 三平台编译验证纳入验收：`GOOS=linux|darwin|windows go build ./...` 全部通过。

## Impact

- 新增：`cmd/strikeye-cli/doctor_platform.go` + 两个构建标签常量文件。
- 修改：`doctorSummary`、`runDoctor`。
- 不改变 IPC/录制在 Windows 上的行为。

# cross-platform Specification

## Purpose
把"哪些能力依赖 Windows"从隐含假设变成可查询事实：三平台统一编译，doctor 显式报告 IPC 传输方式与 HLAE 录制可用性，让 Linux/macOS 用户清楚分析、回放、导出仍然完整可用。

## Requirements

### Requirement: Platform capability reporting

`strikeye-cli doctor` SHALL report host capabilities that depend on the OS:

- 输出 SHALL 含 `platform.os` 与 `platform.arch`。
- 输出 SHALL 含 `platform.ipc_local_socket`，Windows 为 `named_pipe`，其它平台为 `unix_socket`。
- 输出 SHALL 含 `platform.hlae_recording`，仅 Windows 为 `true`。
- 非 Windows 平台 SHALL 附带说明"录制需要 Windows，其它能力可用"。
- 平台能力 SHALL NOT 因 HLAE 缺失而让 `doctor` 失败。

#### Scenario: Windows host

- **WHEN** 在 Windows 上运行 `doctor`
- **THEN** `ipc_local_socket` 为 `named_pipe` 且 `hlae_recording` 为 `true`

#### Scenario: Linux host

- **WHEN** 在 Linux 上运行 `doctor`
- **THEN** `ipc_local_socket` 为 `unix_socket`、`hlae_recording` 为 `false`，`notes` 说明录制仅 Windows

### Requirement: Cross-platform build parity

仓库 SHALL 在 `linux`、`darwin`、`windows` 三个 GOOS 上编译通过全部包：

- 平台差异 SHALL 通过构建标签隔离（IPC 传输、分离进程启动）。
- 平台专属逻辑 SHALL NOT 以运行期字符串判断替代构建标签，除已有的配置探测分支。

#### Scenario: Cross compile

- **WHEN** 依次执行 `GOOS=linux go build ./...`、`GOOS=darwin go build ./...`、`GOOS=windows go build ./...`
- **THEN** 三条命令全部成功且无平台专属编译错误

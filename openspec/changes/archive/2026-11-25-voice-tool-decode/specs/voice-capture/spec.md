# voice-capture

## ADDED Requirements

### Requirement: External decoder delegation

系统 SHALL 能把 WAV 提取委派给外部 Steam 语音解码器：

- SHALL 以 `<tool> <demoPath> <outDir>` 调用，并等待其结束。
- 调用前 SHALL 校验工具路径非空、demo 文件存在，并创建输出目录。
- 返回值 SHALL 只包含本次运行新增的 WAV 文件，按路径排序；调用前已存在的 WAV SHALL NOT 计入。
- 非零退出 SHALL 返回 `ToolExitError`，携带退出码与截断到 240 字符的输出。
- 退出码 10–14 SHALL 分别提示参数错误、demo 无法打开、无语音数据、编码不支持、写盘失败。
- 系统 SHALL NOT 下载或分发解码器二进制，也 SHALL NOT 引入 cgo。

#### Scenario: Successful decode

- **WHEN** 解码器在输出目录写入两条 WAV 并退出 0
- **THEN** 返回两条新 WAV 路径

#### Scenario: Stale tracks ignored

- **WHEN** 输出目录里已有一条上一轮的 WAV
- **THEN** 该文件不出现在本次返回结果中

#### Scenario: No voice data

- **WHEN** 解码器以 12 退出并输出 "no voice data in demo"
- **THEN** 错误包含退出码、提示"no voice data"与解码器输出片段

#### Scenario: Missing inputs

- **WHEN** 工具路径为空或 demo 不存在
- **THEN** 分别返回"decoder tool path is required"与"demo not found"错误，不启动进程

### Requirement: Voice decode CLI

`strikeye-cli voice-decode` SHALL 接受 `--demo-id`、`--tool`、`--out`：

- `--out` 缺省 SHALL 为 `<DataRoot>/media/voice/demo_<id>`。
- 未给 `--demo-id` 时 SHALL 使用第一场已解析 demo。
- SHALL 输出 `demo_id/tool/out_dir/tracks/files/has_voice`。

#### Scenario: Default demo

- **WHEN** 不传 `--demo-id` 而库里有解析完成的 demo
- **THEN** 使用该 demo 并报告其 id

#### Scenario: Report shape

- **WHEN** 解码器产出两条音轨
- **THEN** 输出 `tracks=2`、`has_voice=true` 且 `files` 为两条绝对或相对路径

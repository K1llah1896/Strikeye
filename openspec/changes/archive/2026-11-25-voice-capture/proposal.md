## Why

CS:DM 能从带语音的 demo 里导出每位选手的音轨，Strikeye 目前的回放与分析完全看不到"谁在报点"。语音数据是 P0 缺口，但 CS2 用 Steam 私有编解码器，纯 Go 解码需要 cgo + libopus。

本 change 先解决"数据拿不到"的问题：把 demo 里的语音包按说话人、tick 原样落到一个可重复读取的容器里，并给出每位选手的发言统计。解码交给外部工具，避免把 cgo 依赖塞进主程序。

## What Changes

- 新增 `internal/voice`：解析 demo 的 `CSVCMsg_VoiceData`，写出 `.cs2voice` 容器（magic + 版本 + 逐帧 31 字节头 + 变长负载），并提供 `ReadFrames` 供下游复用。
- 新增 CLI `voice --demo-id`：输出 `media/voice/<demo>.cs2voice`，并打印 `frames/bytes/speakers/has_voice`。
- 无语音 demo（公共匹配的常态）返回 0 帧而非报错，`has_voice=false` 明确表示"这份录没录语音"。
- 不引入 cgo、不引入编解码依赖；WAV 解码路径继续留给外部工具。

## Impact

- 新增：`internal/voice`、`cmd/strikeye-cli/voice.go`。
- 数据落盘：`<DataRoot>/media/voice/`。
- 与 `replay-and-input-stream`、`match-data-export` 无耦合，属于并列的数据源扩展。

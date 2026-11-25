## Why

`voice-capture` 把语音包原样存进了 `.cs2voice`，但"听得到"仍差最后一步：解码。CS2 用 Steam 私有编解码器，纯 Go 解码要么引 cgo（libopus），要么自己实现 Steam 的分帧与解密；两者都意味着把原生依赖塞进主程序。

CS:DM 的做法给出了更稳妥的边界：调用外部解码器进程（`csgo-voice-extractor`）。本 change 把这条路径补成正式能力——Strikeye 负责发现 demo、准备目录、跑解码器、把产物登记成音轨，失败时按解码器的退出码给出可读原因。

## What Changes

- `internal/voice` 新增 `DecodeWithTool(ctx, tool, demoPath, outDir)`：以 `<tool> <demo> <outDir>` 调用外部解码器，返回本次新增的 WAV 列表（已存在的 WAV 不计入）。
- 新增 `ToolExitError{Code, Output}`，把 csgo-voice-extractor 的 10–20 退出码映射成"参数错误/打不开 demo/无语音/编码不支持/写盘失败"。
- 新增 CLI `voice-decode --demo-id --tool --out`：默认输出到 `<DataRoot>/media/voice/demo_<id>/`，未给 `--demo-id` 时取第一场已解析 demo。
- 不绑定具体二进制：任何遵循同一调用约定的解码器都能接。

## Impact

- 新增：`internal/voice/tool.go`、`cmd/strikeye-cli/voice_decode.go`。
- 依赖用户自行安装解码器；程序不下载、不分发二进制（授权与平台差异都交还用户）。
- 无语音 demo 时解码器通常以 12 退出，Strikeye 原样报"无语音数据"。

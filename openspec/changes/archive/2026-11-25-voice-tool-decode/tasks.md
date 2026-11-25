# Tasks

## 1. Decoder bridge

- [x] 1.1 `DecodeWithTool`（参数校验、目录准备、增量 WAV 收集）
- [x] 1.2 `ToolExitError` + 退出码提示
- [x] 1.3 CLI `voice-decode --demo-id --tool --out`

## 2. Verification

- [x] 2.1 单测：正常产出、旧文件不计入、exit 12 提示、缺工具/缺 demo
- [x] 2.2 真实 CLI 跑通（stub 解码器产出 2 条音轨）
- [x] 2.3 `go test ./...`、`openspec validate voice-tool-decode --strict`

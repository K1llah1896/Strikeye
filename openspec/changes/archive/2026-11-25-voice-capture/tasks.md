# Tasks

## 1. Voice container

- [x] 1.1 `internal/voice`: `Frame`/`Speaker`/`Summary`, `WriteFrame`, `ReadFrames`, `ErrInvalidContainer`
- [x] 1.2 31 字节定长头 + 4 字节长度前缀，`maxPayload=4096`
- [x] 1.3 `Extract(ctx, demoPath, io.Writer)` 用 `RegisterNetMessageHandler` 抓 `CSVCMsg_VoiceData`

## 2. CLI

- [x] 2.1 `strikeye-cli voice --demo-id --data-root`
- [x] 2.2 输出到 `media/voice/<demo>.cs2voice`（`.part` + rename）
- [x] 2.3 摘要 JSON：frames/bytes/speakers/has_voice/path

## 3. Verification

- [x] 3.1 往返、非法 magic、版本、超长负载、超限负载单测
- [x] 3.2 无语音 fixture 真实跑通（frames=0, has_voice=false）
- [x] 3.3 `go test ./...`、`openspec validate voice-capture --strict`

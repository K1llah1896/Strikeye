# Tasks

- [x] 加入 `github.com/pion/opus` 并确认无 cgo
- [x] 实现 `voice.DecodeOpus`（分组/排序/解码/混合/WAV 写出）
- [x] 实现 `voice.ParseXOggPackets` 与 `voice.NewContainerWriter` 供测试与导入使用
- [x] `voice-decode` 默认走内置解码，`--tool` 保留外部解码器路径
- [x] `voice` 容器统一命名 `demo_<id>.cs2voice`，解码端兼容旧命名
- [x] 包级测试：混合说话人、跳过非 Opus、WAV 头自洽、坏 Ogg 报错
- [x] CLI 测试：合成捕获端到端、缺捕获报错
- [x] 真实验证：`voice-decode --demo-id 999` + `ffprobe`
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 更新调研报告与 `docs/`

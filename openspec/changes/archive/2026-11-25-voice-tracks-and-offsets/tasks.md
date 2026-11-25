# Tasks

- [x] 容器 v2：帧头 33 字节、偏移 uint32，读取兼容 v1
- [x] 测试：v1 容器读取、超 16 位偏移往返
- [x] `voice.DecodeOpusDir`：分轨 + 混合 + 偏移补静音 + 饱和求和
- [x] `voice-decode` 输出 `tracks` 字段
- [x] 测试：分轨文件存在与时长、晚入场说话人
- [x] 真实验证：两位说话人（0s / 1.5s 入场）→ 1.22s / 2.72s / mixed 3.02s
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 报告 §3 语音行去掉"分轨待做"

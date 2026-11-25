# Tasks

- [x] `audioTrackDraft` + `defaultAudioTracks` + `parseTrackFloat`
- [x] `toAudioTracks`（空行丢弃、音量/偏移归一、上限 4）
- [x] 导出卡片「额外音轨」编辑区（3 行：路径/偏移/音量/静音）
- [x] 导出时写入 `MontageSpec.AudioTracks`
- [x] 测试：空行丢弃与归一、负偏移保留、解析回退、默认行形状
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 报告 §8 更新

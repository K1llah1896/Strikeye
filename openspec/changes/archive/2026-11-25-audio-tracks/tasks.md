# Tasks

- [x] 1.1 `composer.AudioTrack`、`EffectiveVolume`、`ParseAudioTrackSpec`（`@`/`:` 解析、`looksNumeric`、错误分支）
- [x] 1.2 `MixAudioTracks`（adelay/atrim/volume/amix、视频 copy、48k 立体声）
- [x] 1.3 `ExtractAudio`、`AudioTrackExt`
- [x] 1.4 `MontageSpec.AudioTracks` + `audibleTracks` 接线（BGM 之后）
- [x] 1.5 CLI `mixaudio`（`--video --track --out --volume --audio-only --ffmpeg`）
- [x] 2.1 测试：`ParseAudioTrackSpec` 正反例、音量边界、静音过滤、真实 ffmpeg 混音时长/音轨/纯音频导出
- [x] 2.2 真实运行：4s 片段 + 6s 语音（偏移 0.5、音量 0.7）→ 74KB m4a
- [x] 2.3 `go test ./...` 全绿；`openspec validate --all --strict` 通过

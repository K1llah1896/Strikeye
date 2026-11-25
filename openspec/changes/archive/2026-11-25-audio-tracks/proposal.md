# Proposal: 外挂音轨（语音/解说）混音与音频导出

## Why

报告 §4.3 把「音轨同步」列为 P0：CS:DM 允许多条外部音轨（语音提取、解说、BGM）各自设置偏移、音量与静音，并导出纯音频；我们只能在合成时叠一条 `bgm_path`，且音量/淡入淡出写死在 `mixBGM` 里。缺少偏移对齐意味着导入的语音文件永远对不上画面。

## What Changes

- `composer.AudioTrack{Path, OffsetSec, Volume, Mute}` 与 `EffectiveVolume()`（0 取 1.0，上限 4，Mute 归零）。
- `ParseAudioTrackSpec` 解析 `path[@offset[:volume]]`，负偏移表示裁掉音轨头部，`@` 后不是数字时按文件名处理（`a@b.wav` 仍可用），Windows 盘符冒号不受影响。
- `MixAudioTracks`：`atrim/asetpts/adelay/volume` 逐轨处理后 `amix duration=first normalize=0`，视频流 `copy`，音轨统一 48kHz 双声道 AAC。
- `ExtractAudio`：导出纯音频 m4a。
- `MontageSpec.AudioTracks`：合成管线在 BGM 混音之后再叠加额外音轨，偏移相对最终时间线。
- CLI `mixaudio --video --track <spec> --track ... [--out] [--volume] [--audio-only]`。

## Impact

- Affected specs: `ffmpeg-composer`（新增音轨混音要求）
- Affected code: `internal/composer/{audio_mix.go,montage_builder.go}`、`cmd/strikeye-cli/{main.go,mixaudio.go}`

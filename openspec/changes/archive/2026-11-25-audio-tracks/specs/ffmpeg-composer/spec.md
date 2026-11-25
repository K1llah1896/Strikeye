# Spec Delta

## ADDED Requirements

### Requirement: External audio track mixing

合成管线 SHALL 支持在成片上叠加任意数量的外部音轨：

- 每条音轨 SHALL 声明 `Path`、`OffsetSec`、`Volume` 与 `Mute`，音量零值 SHALL 解释为 1.0、上限 4，`Mute` SHALL 使其不参与混音。
- 正偏移 SHALL 用 `adelay` 延迟，负偏移 SHALL 通过 `atrim=start=|offset|` 裁掉音轨头部后再归零。
- 混音 SHALL 以成片时长为准（`amix duration=first`）并关闭自动归一化，视频流 SHALL `copy` 不重编码，音频 SHALL 统一为 48kHz 双声道 AAC。
- 所有音轨都被静音或路径为空时，混音 SHALL 返回错误而不是输出原片。
- `ParseAudioTrackSpec` SHALL 接受 `path`、`path@offset`、`path@offset:volume`；`@` 之后不是数字时 SHALL 仍按路径处理；无效偏移或负音量 SHALL 返回错误。

#### Scenario: Offset aligns comms to the clip
- **WHEN** a 3s clip is mixed with a 5s track at offset 0.5 and volume 0.6
- **THEN** the result keeps a 3s duration and a single stereo AAC stream

#### Scenario: Muted tracks are dropped
- **WHEN** every supplied track is muted
- **THEN** mixing fails with an error instead of writing the input unchanged

### Requirement: Audio-only export

系统 SHALL 能把混音结果导出为纯音频：

- `ExtractAudio` SHALL 输出 160k AAC、48kHz 双声道，并按给定时长截断。
- CLI `mixaudio` SHALL 支持 `--video`、可重复的 `--track`、`--out`、`--volume`、`--audio-only` 与 `--ffmpeg`；未给 `--out` 时按 `<视频名>_mixed.<ext>` 命名，音频导出的扩展名为 `.m4a`。
- `--audio-only` 且输出为音频容器时，系统 SHALL 先混到临时 mp4 再抽取音频，并删除临时文件。

#### Scenario: Audio-only bundle
- **WHEN** `mixaudio --video clip.mp4 --track comms.wav@0.5:0.6 --audio-only` runs
- **THEN** a mixed mp4 and an m4a with the same duration are produced

### Requirement: Montage spec audio tracks

`MontageSpec` SHALL 携带 `audio_tracks`，合成 SHALL 在 BGM 混音之后按顺序叠加这些音轨，偏移相对成片时间线；静音或空路径的音轨 SHALL 被跳过。

#### Scenario: Montage layers comms over BGM
- **WHEN** a montage spec sets both `bgm_path` and one comms track
- **THEN** the exported video contains BGM and comms mixed, and the export still verifies as playable media

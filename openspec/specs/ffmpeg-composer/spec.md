# ffmpeg-composer Specification

## Purpose

Provides hardware-accelerated FFmpeg encoding detection, RGBA-piped key-input HUD overlay rendering, lower-third player name cards, optional FrameMeld post-processing, and montage export orchestration with ffprobe verification.

## Requirements

### Requirement: Hardware Encoder Priority Probing
The system SHALL probe available H.264 encoders in priority order (`h264_nvenc`, `h264_qsv`, `h264_amf`, `libx264`) by verifying encoder presence and running a 1-frame smoke encode, selecting the first working encoder.

#### Scenario: Hardware encoder unavailable falls back to libx264
- **WHEN** `h264_nvenc`, `h264_qsv`, and `h264_amf` fail the 1-frame smoke encode and `libx264` succeeds
- **THEN** the probe selects `libx264` as the active encoder

### Requirement: RGBA-Piped Key HUD Overlay
The system SHALL render transparent RGBA key-input HUD frames (`W`, `A`, `S`, `D`, `Shift`, `Ctrl`, `Space`, `M1`, `M2`, `R`, `E`, `F`) synchronized to `[start_tick, end_tick]` from `InputEdge` timelines and pipe them directly into FFmpeg `overlay` without writing intermediate frame files.

#### Scenario: Key HUD reflects active button state at frame timestamp
- **WHEN** an `InputEdge` sets `KeyW | KeyFire` at tick `T` within `[start_tick, end_tick]`
- **THEN** the composited video frame corresponding to tick `T` displays `W` and `M1` highlighted in the HUD overlay

### Requirement: Lower-Third Player Name Card Rendering
The system SHALL render a lower-third player name card PNG containing the player's avatar (or fallback), nickname, round number, and highlight tag, and composite it only onto real gameplay clip segments.

#### Scenario: Name card excluded from non-gameplay segments
- **WHEN** a montage contains an Intro, a Radar intro segment, a highlight clip, and an Outro
- **THEN** the lower-third name card is overlaid only on the highlight clip segment

### Requirement: Montage Composition and Media Verification
The system SHALL compose montages supporting Intro, hard-cut Radar intro segments before target clips, trimmed highlight clips, `xfade` transitions (deduplicating transition group files), subtitles, BGM mixing with `afade`, and Outro, then verify the exported MP4 via `ffprobe` before recording it in `media_videos` and `montage_exports`.

#### Scenario: Radar intro segment forces hard-cut boundaries and avoids xfade duplication
- **WHEN** a radar intro segment is inserted before clip `i` in a multi-clip montage with `xfade` transitions enabled
- **THEN** the boundaries immediately before and after the radar segment use hard cuts, each `xfade` group output is concatenated exactly once, and the resulting MP4 passes `ffprobe` duration and stream checks before marking `montage_exports` completed

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

### Requirement: Montage multi-track audio editor

合辑导出卡片 SHALL 支持最多三条额外音轨：

- 每行 SHALL 提供路径、偏移（秒，可为负）、音量与静音四个输入。
- 空路径的行 SHALL 被忽略，不进入 `MontageSpec.AudioTracks`。
- 偏移输入为空或非法 SHALL 回退为 0；音量输入为空或非法 SHALL 回退为 1.0；音量大于 4 SHALL 收敛到 4。
- 导出 SHALL 把行转换为 `composer.AudioTrack` 并写入 `MontageSpec.AudioTracks`，由既有混音流程并入输出。
- 编辑器 SHALL 不改变 `BuildMontageSpec` 既有签名与语义。

#### Scenario: Two tracks mixed

- **WHEN** 用户填入 `voice.wav`（偏移 0.5、音量 0.7）与 `crowd.wav`（音量留空）并导出
- **THEN** 规格里出现两条音轨，第二条音量归一为 1.0，输出 MP4 含混合音轨

#### Scenario: Half-typed offset

- **WHEN** 偏移输入框内容是 `-` 或 `0.`
- **THEN** 该行偏移按 0 处理，导出不被阻塞

#### Scenario: Muted track

- **WHEN** 某行勾选静音但填了路径
- **THEN** 该行进入规格且 `Mute=true`，混音时按静音处理

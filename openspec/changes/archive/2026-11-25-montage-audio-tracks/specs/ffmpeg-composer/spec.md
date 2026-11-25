# ffmpeg-composer

## ADDED Requirements

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

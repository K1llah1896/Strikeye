# 合辑多音轨（GUI）

## 为什么

`mixaudio` 与 `MontageSpec.AudioTracks` 已经支持任意多条外部音轨（偏移、音量、静音），但合辑导出卡片只有一格 BGM 路径。POV 语音、现场声这类"第二层声音"在 GUI 里不可达——而它恰好是语音能力补齐后最有价值的用法：把 POV 语音铺在集锦上。

## 变什么

1. 合辑导出卡片新增「额外音轨」编辑区，最多 3 行，每行：路径、偏移（秒，可为负）、音量、静音。
2. 行内偏移/音量用文本输入，解析失败回退默认值（偏移 0、音量 1），不阻塞导出；音量 > 4 收敛到 4。
3. 导出时把行转换成 `composer.AudioTrack` 并写入 `MontageSpec.AudioTracks`，由既有的 `MixAudioTracks` 混入（`amix` + `-c:v copy`）。

## 影响

- capability：`ffmpeg-composer`（ADDED「Montage multi-track audio editor」）。
- 不改 `BuildMontageSpec` 签名（避免动现有测试）；新增 `toAudioTracks` / `renderAudioTrackEditor` 两个纯函数式辅助。

# 内置 Opus 语音解码

## 为什么

`voice-capture` 只把语音负载原样存成 `.cs2voice`，解码完全依赖外部 Steam 语音解码器（CS:DM 的做法是调用 `csgo-voice-extractor` 原生二进制）。这带来两个问题：

- 用户必须自己装一个二进制并知道用法，`voice-decode --tool` 才能出音频，属于"能力没补齐"。
- 报告 §9 一直把"语音内置解码"列在阻塞项，而 CS2 的 `CMsgVoiceAudio.format=2`（OPUS）负载本身就是裸 Opus 包，纯 Go 就能解。

## 变什么

1. 新增纯 Go Opus 解码路径（`github.com/pion/opus`，无 cgo）：把 `format=2` 的帧按说话人分组、按段号/偏移排序后解码为单声道 48kHz PCM，混合成一条 WAV。
2. `voice-decode` 默认走内置解码，`--tool` 仍可回退到外部解码器（Steam 包装格式或用户偏好）。
3. `voice` 的容器统一命名为 `media/voice/demo_<id>.cs2voice`，解码端按 id 直取，并对旧命名做兼容查找。
4. 无语音或非 Opus 帧时给出可行动错误（`no opus-format voice frames found` / `run voice first`），退出码 1，而不是产出空 WAV。

## 影响

- capability：`voice-capture`（新增两个 requirement，不修改既有 requirement）。
- 新依赖 `github.com/pion/opus`（纯 Go，MIT），主程序仍无 cgo。
- 报告 §3/§4.3/§9 关于语音的表述改为"内置解码已落地，Steam 包装格式仍交外部工具"。

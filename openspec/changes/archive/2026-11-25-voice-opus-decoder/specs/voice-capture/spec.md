# voice-capture

## ADDED Requirements

### Requirement: Built-in Opus decoding

系统 SHALL 用纯 Go Opus 解码器把 `format=2` 的语音帧解成可播放音频，不依赖外部二进制：

- `DecodeOpus` SHALL 只解码 `format=OPUS(2)` 的帧，其余帧 SHALL 跳过并计入 `skipped_frames`。
- 帧 SHALL 按说话人分组，并按 `(section, offset, tick)` 升序解码。
- 输出 SHALL 是 44 字节 RIFF 头的 PCM16 单声道 48kHz WAV，所有说话人混合为一条轨。
- 单帧解码失败 SHALL 计入 `decode_errors` 并继续，不得中断整场。
- 无任何可解码帧时 SHALL 返回 `ErrNoOpusFrames`，不得写出空 WAV。
- 实现 SHALL NOT 引入 cgo。

#### Scenario: Mixed speakers

- **WHEN** 用两位说话人的 Opus 帧解码
- **THEN** 返回两条说话人记录，WAV 头部采样率为 48000，数据长度等于样本数×2

#### Scenario: Non-Opus frames

- **WHEN** 容器里全是 `format=0` 的 Steam 包装帧
- **THEN** 返回 `ErrNoOpusFrames`，`skipped_frames` 等于帧数，且不创建输出文件

### Requirement: Voice decode command defaults to built-in

`strikeye-cli voice-decode` SHALL 在未指定 `--tool` 时使用内置解码，并 SHALL 在指定工具时回退到外部解码器：

- 无 `--tool` 时 SHALL 读取 `media/voice/demo_<id>.cs2voice`，输出 `<out>/mixed.wav`，并打印 `engine=opus`、`frames`、`samples`、`seconds`、`skipped_frames`、`speakers`。
- 找不到捕获文件时 SHALL 报错并提示先运行 `voice`，退出码非 0。
- 旧命名捕获（`<demo 文件名>.cs2voice`）SHALL 仍可被找到。
- `voice` 命令 SHALL 统一把容器写成 `media/voice/demo_<id>.cs2voice`。

#### Scenario: Decode synthesized capture

- **WHEN** 在临时 DataRoot 里放一份由真实 Opus 包构成的 `demo_7.cs2voice` 并执行 `voice-decode --demo-id 7`
- **THEN** 输出 JSON 的 `engine` 为 `opus`、`has_voice` 为真、`seconds` 与源时长相差在 0.3 秒内，且 WAV 文件存在

#### Scenario: Missing capture

- **WHEN** DataRoot 里没有该 demo 的捕获文件
- **THEN** 命令返回错误并提示 `run voice first`

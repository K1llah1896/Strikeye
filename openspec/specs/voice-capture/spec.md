# voice-capture Specification

## Purpose

把 demo 中的 Steam 语音包按说话人与 tick 原样保存为版本化容器，让回放分析能回答"这场谁在报点"，同时避免把 cgo 与私有编解码器带进主程序（解码交由外部工具）。

## Requirements

### Requirement: Voice packet capture

系统 SHALL 能把 demo 中的 `CSVCMsg_VoiceData` 原样保存为可重复读取的容器：

- 每帧 SHALL 记录 tick、XUID、格式、采样率、序列号、段号、负载长度与负载。
- 容器 SHALL 以 `SKVO` magic 与版本号开头，读到未知 magic 或版本时 SHALL 返回 `ErrInvalidContainer`。
- 单帧负载 SHALL 限制在 4096 字节内；写入超限与读取超限 SHALL 都报错而不是截断。
- 系统 SHALL 提供 `ReadFrames` 读回容器，用于校验与下游再处理。
- demo 没有语音包时 SHALL 写出仅含文件头的容器，`frames=0`、`has_voice=false`，且退出码为 0。
- 系统 SHALL NOT 引入 cgo 或原生编解码依赖。

#### Scenario: Round trip

- **WHEN** 写入三帧不同说话人/采样率的语音包后再读回
- **THEN** 帧数、tick、XUID、采样率、偏移与负载逐字节一致

#### Scenario: Foreign file

- **WHEN** 读取一个不以 `SKVO` 开头的文件
- **THEN** 返回 `ErrInvalidContainer`，不 panic

#### Scenario: Corrupt length prefix

- **WHEN** 帧长度前缀为 `0xFFFFFFFF`
- **THEN** 读取报错，不分配该长度的内存

#### Scenario: Demo without voice

- **WHEN** 用仓库内的无语音 fixture 执行 `strikeye-cli voice`
- **THEN** 输出 `frames=0`、`speakers=[]`、`has_voice=false`，并写出 6 字节空容器

### Requirement: Voice activity summary

`strikeye-cli voice` SHALL 输出语音摘要：

- SHALL 列出每位说话人的帧数与字节数，按帧数降序、XUID 升序排列。
- SHALL 输出写入路径 `<DataRoot>/media/voice/<demo>.cs2voice`。
- SHALL 用 `.part` 临时文件写入后 rename，失败时删除临时文件。

#### Scenario: Speaker ordering

- **WHEN** 两个 XUID 分别有 3 帧与 5 帧
- **THEN** 摘要中 5 帧的说话人排在前面

#### Scenario: Failure leaves no partial file

- **WHEN** 解析中途失败
- **THEN** `.cs2voice` 与 `.part` 都不残留

### Requirement: External decoder delegation

系统 SHALL 能把 WAV 提取委派给外部 Steam 语音解码器：

- SHALL 以 `<tool> <demoPath> <outDir>` 调用，并等待其结束。
- 调用前 SHALL 校验工具路径非空、demo 文件存在，并创建输出目录。
- 返回值 SHALL 只包含本次运行新增的 WAV 文件，按路径排序；调用前已存在的 WAV SHALL NOT 计入。
- 非零退出 SHALL 返回 `ToolExitError`，携带退出码与截断到 240 字符的输出。
- 退出码 10–14 SHALL 分别提示参数错误、demo 无法打开、无语音数据、编码不支持、写盘失败。
- 系统 SHALL NOT 下载或分发解码器二进制，也 SHALL NOT 引入 cgo。

#### Scenario: Successful decode

- **WHEN** 解码器在输出目录写入两条 WAV 并退出 0
- **THEN** 返回两条新 WAV 路径

#### Scenario: Stale tracks ignored

- **WHEN** 输出目录里已有一条上一轮的 WAV
- **THEN** 该文件不出现在本次返回结果中

#### Scenario: No voice data

- **WHEN** 解码器以 12 退出并输出 "no voice data in demo"
- **THEN** 错误包含退出码、提示"no voice data"与解码器输出片段

#### Scenario: Missing inputs

- **WHEN** 工具路径为空或 demo 不存在
- **THEN** 分别返回"decoder tool path is required"与"demo not found"错误，不启动进程

### Requirement: Voice decode CLI

`strikeye-cli voice-decode` SHALL 接受 `--demo-id`、`--tool`、`--out`：

- `--out` 缺省 SHALL 为 `<DataRoot>/media/voice/demo_<id>`。
- 未给 `--demo-id` 时 SHALL 使用第一场已解析 demo。
- SHALL 输出 `demo_id/tool/out_dir/tracks/files/has_voice`。

#### Scenario: Default demo

- **WHEN** 不传 `--demo-id` 而库里有解析完成的 demo
- **THEN** 使用该 demo 并报告其 id

#### Scenario: Report shape

- **WHEN** 解码器产出两条音轨
- **THEN** 输出 `tracks=2`、`has_voice=true` 且 `files` 为两条绝对或相对路径

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

### Requirement: Wide voice offsets

容器 SHALL 用 32 位字段保存每个语音包的非压缩样本偏移，避免长句时间线回绕：

- 容器版本 SHALL 升为 2，帧头 SHALL 为 33 字节，偏移写在第 29–32 字节。
- 读取 SHALL 同时接受版本 1（31 字节帧头、16 位偏移）与版本 2。
- 读取版本 1 时 SHALL 把 16 位偏移原样扩展为 32 位，不得报错。
- 写入 SHALL 一律使用版本 2。

#### Scenario: Long offset round trip

- **WHEN** 写入偏移 3,000,000 的帧再读回
- **THEN** 偏移逐位一致，不再回绕到 16 位范围

#### Scenario: Legacy container

- **WHEN** 读取版本 1 容器（偏移 65535）
- **THEN** 解析成功，帧偏移为 65535

### Requirement: Per-speaker voice tracks

内置解码 SHALL 能按说话人导出独立音轨，并与 demo 时间线对齐：

- 每个说话人 SHALL 写一条 `speaker_<xuid>.wav`，并写 `mixed.wav` 作为混合轨。
- 音轨 SHALL 按帧偏移补静音，使样本在时间线上的位置与 demo 一致。
- 偏移小于当前写入位置或超出 60 秒上限时 SHALL 追加到当前位置，不得报错或分配巨量内存。
- 混合轨 SHALL 用饱和求和，避免重叠语音溢出成杂音。
- `voice-decode` 输出 SHALL 包含每个说话人的文件路径、时长与帧数。

#### Scenario: Two speakers, one joins late

- **WHEN** 说话人 A 从 0 秒开始、B 从 1.5 秒开始，两人各 1.2 秒语音
- **THEN** A 的轨道约 1.2 秒、B 的轨道约 2.7 秒（含 1.5 秒前置静音），混合轨约等于 B 的长度

#### Scenario: Missing capture

- **WHEN** DataRoot 里没有该 demo 的捕获文件
- **THEN** 命令报错并提示先运行 `voice`

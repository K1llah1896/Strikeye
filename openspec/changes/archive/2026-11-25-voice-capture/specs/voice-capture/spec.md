# voice-capture

## ADDED Requirements

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

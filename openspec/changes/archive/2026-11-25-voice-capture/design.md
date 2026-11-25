## Context

CS2 demo 的语音消息是 `netmessages.proto` 里的 `CSVCMsg_VoiceData`，负载是 Steam voice（Opus 变体 + 自定义分帧）。demoinfocs 已注册该消息（`s2_commands.go`），但没有高层事件，需要 `RegisterNetMessageHandler`。

实测仓库内两份 fixture 的语音包数为 0——公共匹配 demo 默认不记录语音，因此功能必须在"没有语音"时安静降级。

## Goals / Non-Goals

**Goals**

- 抓取全部语音帧，保留说话人（XUID）、tick、序列号、采样率、段号与负载。
- 容器可流式写、可重复读，版本化以免未来加字段破坏旧文件。
- 输出说话人统计，供 UI/报告判断"这场有没有语音、谁说得最多"。

**Non-Goals**

- 不做解码（需要 Steam codec / libopus）。
- 不做语音活动检测、不做字幕、不做音频混音。
- 不修改解析流水线：`voice` 是独立的第二遍解析，不拖慢 `parse`。

## Decisions

1. **独立容器而非 WAV**：没有解码器就写不出合法 WAV；原始包容器保证信息不丢，未来接解码器只需读容器。
2. **31 字节定长头 + 4 字节长度前缀**：字段对齐简单，解析无需 schema；`maxPayload=4096` 防止损坏文件把内存打爆。
3. **magic `SKVO` + `FormatVersion=1`**：与 `.cs2rpl`、`.tri` 一致的自描述风格，读到未知版本直接报 `ErrInvalidContainer`。
4. **payload 上限 4096**：Steam 语音单包远小于此值，超过即视为损坏。
5. **CLI 用 `.part` + rename**：中途失败不会留下半截文件，和几何/导航下载保持一致。
6. **`has_voice` 而非 `decodable`**：字段回答的是"这份 demo 有没有语音数据"，不是"我们能不能解码"。

## Risks / Trade-offs

- 第二遍解析多花一次解析时间（约 15s/图）；只在显式调用 `voice` 时发生。
- 容器不压缩：一小时语音约数 MB，可接受；如需压缩可直接 gzip 外层。
- Steam 若更改语音消息结构，`Format`/`SampleRate` 可能失真，但负载仍原样保留。

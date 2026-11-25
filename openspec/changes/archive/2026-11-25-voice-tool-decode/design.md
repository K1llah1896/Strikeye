## Context

容器 `.cs2voice` 已经保留了负载、XUID、tick 与采样率。外部解码器的输入约定不一致，但社区最常用的是 `csgo-voice-extractor <demo> <outDir>`，输出 `<xuid>.wav`。Strikeye 只需要把约定固定成契约并解释失败。

## Goals / Non-Goals

**Goals**

- 一条命令从 demo 到 WAV 目录，且明确报告"这次产出了几条音轨"。
- 失败可诊断：区分"工具不存在"、"demo 打不开"、"没有语音"、"编码不支持"。
- 不引入 cgo、不下载二进制。

**Non-Goals**

- 不做音频混音、不生成 `<demoID>/<steamID>.wav` 之外的命名策略（解码器决定文件名）。
- 不做音轨偏移校准（属于播放器侧）。
- 不解析 `.cs2voice` 容器做解码：解码器直接读 demo，容器用于统计与未来自研解码。

## Decisions

1. **外部进程而非 cgo**：解码器崩溃/缺失不会拖垮主程序，也避免交叉编译噩梦。
2. **`ToolExitError` 携带原始输出（截断 240 字符）**：用户能直接看到解码器自己的说明。
3. **只报告"新增"的 WAV**：目录里已有旧音轨时不重复计入，便于增量重跑。
4. **默认输出 `<DataRoot>/media/voice/demo_<id>`**：与 `media/` 下其它产物一致，便于 UI 将来挂播放器。
5. **工具路径必填**：不做 PATH 搜索，避免误执行同名程序。

## Risks / Trade-offs

- 契约绑定：改约定即破坏兼容；已在文档声明"任何遵循 `<tool> <demo> <dir>` 的实现都可接"。
- 解码器可能长时间运行：受 `ctx` 控制，用户 Ctrl-C 会传下去。

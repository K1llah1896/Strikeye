# replay-and-input-stream

## ADDED Requirements

### Requirement: Voice activity timeline

回放页 SHALL 展示每回合谁在什么时候说话：

- 语音包 SHALL 按说话人聚合成说话段；相邻语音包间隔不超过 32 tick（0.5 秒 @64 tick）时 SHALL 合并为同一段。
- 时间轴 SHALL 以回合冻结结束时刻为 0，宽度固定 60 字符，用 `█` 表示发言、`·` 表示静默。
- 说话段跨出回合边界时 SHALL 裁剪到回合范围而不是丢弃。
- 时间轴 SHALL 按说话总时长降序排列，名字优先取选手名，缺失时回退 `XUID <id>`。
- 无语音捕获时 SHALL 显示说明文案（Valve 匹配 demo 不录语音，POV demo 可用 `voice` 提取），不得报错。
- CLI SHALL 提供 `voice-activity --demo-id [--round]`，输出每个说话段相对回合的起止秒、时长与帧数，以及说话人数量与总时长。
- `voice-decode --xuid <id64>` SHALL 只导出该说话人的轨道，且在该说话人未发言时报错。

#### Scenario: Two bursts in one round

- **WHEN** 某选手在回合内先连续说 41 包、停顿 1 秒、再说 20 包
- **THEN** 时间轴显示两段独立的 `█`，段数计数为 2

#### Scenario: No voice in the demo

- **WHEN** demo 的语音捕获为空
- **THEN** 卡片显示"本场没有语音捕获"，CLI 输出 `has_voice=false` 且退出码为 0

#### Scenario: Speaker filter

- **WHEN** 对 `voice-decode` 传入未发言的 `--xuid`
- **THEN** 命令报错 "speaker <id> did not talk in this capture"

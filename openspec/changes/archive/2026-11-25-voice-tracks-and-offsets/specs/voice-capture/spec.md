# voice-capture

## ADDED Requirements

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

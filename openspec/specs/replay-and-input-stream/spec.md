# replay-and-input-stream Specification

## Purpose

Encodes and reads the CS2RPL01 binary cache containing 32Hz 2D replay frames, grenade/smoke/inferno geometry, and embedded raw-tick player input telemetry without fabricating mouse trajectories.

## Requirements

### Requirement: 32Hz replay and grenade geometry capture
During the single-pass parse, the collector SHALL sample living player poses and state at `stride = max(1, round(tickRate / 32.0))` ticks alongside grenade projectile paths, smoke detonation/dissipation windows, and active inferno 2D convex hulls.

#### Scenario: 32Hz frames and utility geometry persisted to CS2RPL01
- **WHEN** a 64-tick demo completes single-pass parsing
- **THEN** `<DataRoot>/cache/replay_32hz/<sha256>.rpl` is written atomically with `CS2RPL01` magic, round index offsets, 32Hz player frames, and grenade/smoke/inferno chunks, and `demos.replay_cache_path` is updated

### Requirement: Embedded raw-tick player input stream and capability reporting
The input collector SHALL extract button edge transitions (`W, A, S, D, Jump, Crouch, Walk, Reload, Fire, Scope, Use, Inspect, Scoreboard`) from `CSVCMsg_UserCmd` with entity `m_nButtons` fallback, embed them as a raw-tick chunk inside `CS2RPL01`, and set `demos.input_data_capability` to `full_usercmd`, `entity_buttons_only`, or `none`.

#### Scenario: UserCmd stripped in platform demo
- **WHEN** a demo lacks `CSVCMsg_UserCmd` messages but exposes entity `m_nButtons`
- **THEN** `demos.input_data_capability` is recorded as `entity_buttons_only` and button transitions are still embedded in `CS2RPL01`

#### Scenario: Input telemetry absent without fabricated mouse paths
- **WHEN** neither `CSVCMsg_UserCmd` nor `m_nButtons` is available in the demo
- **THEN** `demos.input_data_capability` is recorded as `none` and the system MUST NOT synthesize physical mouse trajectories from view angles or movement

### Requirement: Indexed CS2RPL01 round-level random access
The `CS2RPL01` reader SHALL validate file headers and section offsets so callers can load metadata, a single round's 32Hz replay/grenade frames, or a tick range's input transitions without decoding the entire file.

#### Scenario: Single-round replay and input slice read
- **WHEN** a reader requests round 5 from an `.rpl` file
- **THEN** only the header, round index, and round 5 chunks are decoded and returned

### Requirement: Combat event replay chunk and version compatibility
The replay collector SHALL append a combat chunk (shots, deaths, bomb actions) to each round payload and SHALL bump the `CS2RPL01` format version while remaining able to read older caches.

#### Scenario: v2 cache carries combat events
- **WHEN** a demo is parsed by the current parser version
- **THEN** each round payload includes shot, death, and bomb event chunks alongside poses and utility effects, and the header records format version 2

#### Scenario: v1 cache remains readable
- **WHEN** the reader opens a `CS2RPL01` file written with format version 1
- **THEN** it returns the legacy rounds with empty combat event slices instead of failing

#### Scenario: Unknown newer version is rejected
- **WHEN** the reader encounters a format version greater than the supported version
- **THEN** it returns `ErrUnsupportedReplayVer`

#### Scenario: Corrupt combat chunk is rejected
- **WHEN** a v2 payload ends before a declared combat event record is complete
- **THEN** decoding returns `io.ErrUnexpectedEOF`

### Requirement: Freehand stroke storage

系统 SHALL 持久化自由笔迹：

- 迁移 SHALL 建立 `map_strokes(demo_id, round_number, color, points_json, created_at)` 并索引 `(demo_id, round_number)`。
- 笔迹点 SHALL 存为归一化 `[0,1]` 坐标，写入时 SHALL 裁剪到该区间。
- 少于两个点的笔迹 SHALL 被拒绝；缺少 demo id SHALL 被拒绝。
- 颜色 SHALL 限定 amber/red/green/blue，非法值归为 amber。
- 读取时坏 JSON 行 SHALL 返回空点集而不是让整个查询失败。
- SHALL 支持按 id 删除与按 (demo, round) 批量删除。

#### Scenario: Coordinates are clamped

- **WHEN** 写入点 `(-1, 2)`
- **THEN** 存储为 `(0, 1)`

#### Scenario: Single point rejected

- **WHEN** 写入只含一个点的笔迹
- **THEN** 返回错误且不落库

#### Scenario: Corrupt row tolerated

- **WHEN** `points_json` 不是合法 JSON
- **THEN** `List` 仍返回该行，`points` 为空

### Requirement: Freehand drawing on the 2D canvas

系统 SHALL 在 2D 回放画布上支持自由绘制：

- 画布 SHALL 接受归一化笔迹并按画布尺寸缩放绘制，线宽 3、圆头，绘制在选手与标记之后。
- 少于两点的笔迹 SHALL NOT 绘制。
- 笔刷开启时，画布 SHALL 在指针位于画布内且主键按下时每帧回调归一化坐标与按下状态；指针拖出画布仍保持按下时 SHALL 继续回调。
- 拖动期间采样点 SHALL 与上一点距离小于 0.004 时跳过。
- 松开主键 SHALL 把整条笔迹一次性写入存储并刷新画布。

#### Scenario: Stroke changes pixels

- **WHEN** 画布设置一条从 (0.1,0.1) 到 (0.9,0.9) 的三点笔迹并重绘
- **THEN** 输出图像与无笔迹时存在像素差异

#### Scenario: Sampling throttles near-duplicate points

- **WHEN** 连续回调 (0.1,0.1) 与 (0.1005,0.1005)
- **THEN** 只记录第一个点

#### Scenario: Release persists

- **WHEN** 拖动后松开主键
- **THEN** 数据库新增一条笔迹，内存中的待写点清空，画布显示该笔迹

### Requirement: Stroke CLI

系统 SHALL 提供 `strikeye-cli strokes --demo-id <id>`：

- SHALL 输出该 demo 的笔迹列表与数量。
- `--round n` SHALL 只返回该回合的笔迹。
- `--clear` SHALL 删除匹配的笔迹并输出删除数量。
- 缺少 `--demo-id` SHALL 返回错误。

#### Scenario: Clear one round

- **WHEN** 以 `--demo-id 2 --round 3 --clear` 运行
- **THEN** 返回 `{"removed": n, "round": 3}`

### Requirement: Kill feed overlay

2D 回放画布 SHALL 能把最近的击杀渲染成屏内信息条：

- 只 SHALL 显示当前 tick 之前、且在 6 秒窗口内的击杀，最多 4 条，按时间升序排列（最新在底部）。
- 每条 SHALL 显示击杀者与死者的选手名，爆头时 SHALL 追加红色标记。
- 武器名 SHALL 由击杀者最接近的弹道事件推断（±64 tick 内），推断不到时 SHALL 省略。
- 缺少 slot→姓名映射时 SHALL 回退为 `#<slot>`。
- 超过窗口的击杀 SHALL 完全不再绘制（与关闭该图层的结果逐像素一致）。

#### Scenario: Recent kills are listed
- **WHEN** a round has a kill 4 ticks before the rendered frame
- **THEN** the canvas draws a feed row naming killer and victim

#### Scenario: Expired kills disappear
- **WHEN** the rendered frame is 6+ seconds after the last kill
- **THEN** the frame is identical to one rendered with the kill feed disabled

### Requirement: Lower-level alignment controls

2D 画布 SHALL 支持把下层雷达按偏移与透明度合成为底图：

- `SetLowerView(offsetX, offsetY, opacity)` SHALL 钳制偏移到 ±512、透明度到 (0,1]，并在参数变化时重建背景缓存。
- 默认参数（0,0,1）下的渲染 SHALL 与未设置时逐像素一致。
- 分析页 SHALL 在回放与热力图两个面板提供下层 X/Y 偏移与不透明度控件，以及「重置」与「下层半透明」快捷按钮。
- 改动 SHALL 立即持久化，并在选中 demo 时按地图名载入。

#### Scenario: Offset shifts the underlay
- **WHEN** the lower view is moved by 24/-16 px
- **THEN** the rendered backdrop changes

#### Scenario: Reset restores the original backdrop
- **WHEN** offsets are returned to 0,0 with opacity 1
- **THEN** the rendered backdrop matches the pre-adjustment frame byte for byte

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

### Requirement: Shared kill weapon attribution

武器归属 SHALL 由单一实现提供：

- `internal/parser` SHALL 暴露 `KillWeaponWindowTicks`（64）与 `NearestShotWeapon(round, slot, tick)`，返回同槽位窗口内最近一发、已剥离 `weapon_` 前缀的武器名。
- 2D 击杀信息条与击杀上下文（含导出 `weapon` 列）SHALL 使用该函数，不得各自实现。
- 窗口外或该槽位没有射击记录时 SHALL 返回空字符串。

#### Scenario: Same weapon in canvas and export

- **WHEN** 同一次击杀同时出现在 2D 击杀信息条与 `match_<id>_kills.csv`
- **THEN** 两处武器名一致

#### Scenario: Knife kill

- **WHEN** 击杀由刀完成（无 `weapon_fire`）
- **THEN** 武器为空字符串，UI 隐藏武器图标，导出列为空

### Requirement: Route trails

2D 回放 SHALL 提供推进路线图层：

- 每 8 tick（64 tick 下 4Hz）采样一次存活选手位置，并在当前帧额外补一个采样点。
- 折线 SHALL 按队伍着色（T 橙、CT 蓝），透明度低到不遮挡地图细节，当前位置画点。
- 少于两个采样点的选手 SHALL 不绘制。
- 图层 SHALL 可开关且默认开启；关闭后同帧渲染结果不得包含轨迹像素。
- 该图层表示"实际走过的路径"，与 `strikeye-cli navpath` 的 navmesh 最短路径互为对照而非替代。

#### Scenario: Toggle route layer
- **WHEN** 用户取消 2D 回放页的「推进路线」勾选
- **THEN** 同一帧重新渲染后不再出现轨迹，其余图层不受影响

#### Scenario: Stationary player
- **WHEN** 某选手整回合位置不变
- **THEN** 只有末点被画出，不产生零长度折线

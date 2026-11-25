# visibility-metrics Specification

## Purpose
利用本地缓存的地图几何数据计算“首次伤害前持续可见时长”（TTD），为对枪效率分析提供视线依据，并在几何缺失时优雅降级。

## Requirements

### Requirement: Visibility sampling
The parser SHALL sample alive player eye positions at the replay sampling rate while a round is active.

#### Scenario: Ring buffer window
- **WHEN** a round runs longer than five seconds
- **THEN** samples older than five seconds are discarded before the next damage event is evaluated

#### Scenario: Dead players excluded
- **WHEN** a player is dead
- **THEN** no visibility sample is recorded for them

### Requirement: Time-to-damage measurement
The parser SHALL record at most one TTD sample per round, attacker, and victim pair, measured as the continuous visible run before the first damage.

#### Scenario: Victim exposed briefly
- **WHEN** the victim was visible for 200 ms before the first hit
- **THEN** the sample is approximately 200 ms

#### Scenario: No line of sight
- **WHEN** the sight line was blocked for the whole window (for example a wallbang)
- **THEN** no sample is recorded for that pair

#### Scenario: Same team or self damage
- **WHEN** attacker and victim are the same player or share a team
- **THEN** no sample is recorded

### Requirement: Geometry-optional parsing
Parsing SHALL succeed when map geometry is absent.

#### Scenario: Missing geometry file
- **WHEN** `<DataRoot>/cache/map_geometry/<map>.tri` does not exist
- **THEN** the demo parses normally and TTD samples remain zero

### Requirement: TTD persistence and surfacing
The system SHALL persist average TTD and sample count per player and expose them in analysis and CSV export.

#### Scenario: Storage
- **WHEN** metrics include TTD samples
- **THEN** `match_players.ttd_avg_ms` and `ttd_samples` hold the average and count

#### Scenario: CSV export
- **WHEN** players are exported to CSV
- **THEN** the header and rows include `ttd_avg_ms` and `ttd_samples`

#### Scenario: Analysis drawer
- **WHEN** a player with TTD samples is expanded
- **THEN** the drawer shows the average TTD and sample count

### Requirement: Optics-aware visibility

每一帧可见性判定 SHALL 同时考虑遮挡、攻击者致盲与攻击者水平视野：

- 采样 SHALL 记录每位存活选手的眼角 yaw（度，0=+X，逆时针正）与剩余致盲秒数。
- 一帧 SHALL 仅在"无遮挡 且 攻击者剩余致盲 ≤ 0.1 秒 且 受害者位于攻击者 53° 水平半角内"时计为可见。
- 任一条件不满足 SHALL 中断向后回溯的可见游程。
- 攻击者与受害者水平距离小于 8 单位时 SHALL 跳过 FOV 判定。
- FOV 半角 SHALL 可配置，默认值为 53。

#### Scenario: Facing the victim

- **WHEN** 攻击者 yaw 正对且无遮挡
- **THEN** 该帧计为可见

#### Scenario: Facing away

- **WHEN** 攻击者 yaw 与受害者方向相反（180°）
- **THEN** 游程为空，不记录 TTD 样本

#### Scenario: Narrow cone

- **WHEN** 受害者偏离 30°，FOV 半角配置为 20°
- **THEN** 不计为可见；配置为 53° 时计为可见

#### Scenario: Attacker blinded

- **WHEN** 攻击者剩余致盲 1.4 秒
- **THEN** 不记录 TTD；剩余致盲降到 0.05 秒后恢复记录

#### Scenario: Point blank

- **WHEN** 两人眼位相距不足 8 单位
- **THEN** 只按遮挡判定，不做 FOV 剔除

### Requirement: Scope-aware cone

可见性采样 SHALL 记录每位存活选手的开镜等级，并据此收窄水平视野半角：

- 回放姿势 SHALL 记录 `Zoom`（0=未开镜、1=一级、2=二级）。
- 采样 SHALL 从 `ActiveWeapon().ZoomLevel()` 取开镜等级，未开镜记为 0。
- 判定半角 SHALL 为：未开镜沿用配置值（默认 53°）、一级开镜 20°、二级开镜 7.5°。
- 击杀光学 SHALL 输出 `attacker_zoom`、`victim_zoom`，且 `in_fov` 用攻击者开镜半角、`victim_sees_killer` 用受害者开镜半角。
- 回放格式版本 SHALL 升为 3；读取 v2 文件时 `Zoom` SHALL 读回 0，且不得因缺少 zoom 字节而报错。

#### Scenario: Scoped attacker loses a wide target

- **WHEN** 受害者偏离攻击者视线 30°，攻击者二级开镜
- **THEN** 该帧不计为可见，TTD 游程中断

#### Scenario: Half zoom keeps a narrow target

- **WHEN** 受害者偏离 15°，攻击者一级开镜
- **THEN** 该帧计为可见

#### Scenario: Legacy replay file

- **WHEN** 读取一个 v2 版 `.cs2rpl`
- **THEN** 解析成功，所有姿势的 `Zoom` 为 0

#### Scenario: Zoom round trip

- **WHEN** 写入含 `Zoom=2` 姿势的回合再读回
- **THEN** 版本为 3，`Zoom`、yaw 与武器名逐项一致

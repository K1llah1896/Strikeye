# visibility-metrics

## ADDED Requirements

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

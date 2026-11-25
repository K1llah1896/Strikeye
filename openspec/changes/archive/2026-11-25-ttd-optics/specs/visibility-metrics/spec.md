# visibility-metrics

## ADDED Requirements

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

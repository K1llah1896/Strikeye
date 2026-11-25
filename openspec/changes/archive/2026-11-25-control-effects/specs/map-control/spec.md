# Spec Delta

## Purpose

在可达性控制模型之上叠加烟雾与火焰效果，让控制曲线反映"被封住的区域不算谁的"，并让火焰阻断推进路径。

## MODIFIED Requirements

### Requirement: Reachability control model

系统 SHALL 以截断 Dijkstra 计算每支队伍的地图控制，并扣除烟雾与火焰的影响：

- 对每帧的存活选手，按队伍分别做多源 Dijkstra，代价为区域质心间距离，截断半径默认 1100 世界单位。
- 两队都能到达且距离差小于争夺阈值（默认 200 单位）的区域记为争夺；否则归到达更近的一方；仅一队可达归该队；都不可达记中立。
- 生效烟雾球（半径默认 144 单位、Z 容差 144）覆盖的区域 SHALL 不计入任何一方的归属与净值。
- 生效火焰凸包覆盖的区域 SHALL 不计入归属，且 SHALL 不参与 Dijkstra 展开，使任何队伍都无法"穿过"火墙。
- 净值定义为 `(CT区域数 - T区域数) / 已分类区域数`，落在 `[-1, 1]`；遮挡区域不进入分子或分母。
- 每帧 SHALL 输出遮挡区域数 `blocked`。
- 采样从回放回合的 `freeze_end_tick` 开始，默认每 5 秒一次，避免热身数据污染曲线。

#### Scenario: Both teams equidistant from an area

- **WHEN** 两队到某区域的距离差小于争夺阈值
- **THEN** 该区域计入争夺数，不计入任何一方的区域数与净值分子

#### Scenario: Smoke removes an area from ownership

- **WHEN** 一颗烟雾弹在采样时刻生效且覆盖某区域质心
- **THEN** 该区域计入 `blocked`，不计入 T/CT/争夺/中立，也不进入净值

#### Scenario: Fire also cuts traversal

- **WHEN** 一道火在采样时刻生效并覆盖两支队伍之间的唯一通路区域
- **THEN** 该区域计入 `blocked`，且对面队伍的可达距离不会穿过该区域

#### Scenario: Effects can be disabled for comparison

- **WHEN** 用户以 `--effects=false` 运行 `mapcontrol`
- **THEN** 输出的 `blocked` 全为 0，净值回到纯可达性模型的结果

#### Scenario: Sampling starts at freeze time

- **WHEN** 回放的回合起点包含 88 秒热身
- **THEN** 控制曲线的第一个采样点位于 freeze time，时间轴从 0 开始

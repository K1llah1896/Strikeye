# Spec Delta

## Purpose

在几何上下文之上补一层光学上下文：击杀者是否瞄着对方、对方是否看着击杀者、双方是否被闪。

## MODIFIED Requirements

### Requirement: Per-kill geometry annotation

系统 SHALL 为回放缓存中的每次击杀计算几何与光学上下文：

- 交火距离 SHALL 按击杀者眼位（位置 + 64）到受害者胸口（位置 + 45）的欧氏距离计算。
- 在提供地图几何时，系统 SHALL 用 BVH 判定该连线是否被遮挡，并输出 `visible`。
- 在缺少地图几何时，系统 SHALL 输出 `geometry_checked=false`，距离与烟雾判定仍然可用，且不得报错。
- 系统 SHALL 判定连线是否穿过击杀 tick 生效的烟雾球（半径 144 单位，中心抬高 32 单位），输出 `smoke_blocked`。
- 在存在回放帧时，系统 SHALL 用击杀 tick 之前最近的帧计算 `aim_off_deg`（击杀者视线与连线的夹角）、`in_fov`（半视角 53°）、`victim_sees_killer`、`attacker_blind` 与 `victim_blind`（剩余致盲 > 0.1 秒）。
- 没有可用回放帧时，系统 SHALL 输出 `optics_checked=false`，且不得把缺失当成"看不见"。
- 汇总 SHALL 包含平均交火距离、被遮挡击杀占比、穿烟击杀占比、视野内占比、背身击杀占比与致盲击杀数。
- 击杀列表 SHALL 按回合号与 tick 升序排列。

#### Scenario: Kill through a wall

- **WHEN** 击杀者与受害者之间被墙体隔断且提供了该图几何
- **THEN** 该次击杀的 `visible` 为 false，并计入"被遮挡"比例

#### Scenario: Kill through smoke

- **WHEN** 一条击杀连线穿过在击杀 tick 生效的烟雾
- **THEN** `smoke_blocked` 为 true，并计入穿烟比例

#### Scenario: No geometry cached

- **WHEN** `cache/map_geometry` 中没有该图数据
- **THEN** `killctx` 仍输出距离与烟雾判定，`geometry_used` 与 `geometry_checked` 为 false，退出码为 0

#### Scenario: Ambush kill

- **WHEN** 受害者的视线方向与"受害者→击杀者"连线的夹角超过半视角
- **THEN** `victim_sees_killer` 为 false，该击杀计入 `ambush_pct`

#### Scenario: Blinded killer

- **WHEN** 击杀 tick 时击杀者的剩余致盲时间大于 0.1 秒
- **THEN** `attacker_blind` 为 true，并计入 `blind_kills`

#### Scenario: Kill table shows geometry

- **WHEN** 用户打开区域控制页的击杀区位表
- **THEN** 每行显示交火距离与视线状态（可见 / 遮挡 / 遮挡·烟 / 未知）

#### Scenario: Kill table shows optics

- **WHEN** 用户打开区域控制页的击杀区位表
- **THEN** 视线列以 `·背身` 标记未被看到的受害者、以 `·盲` 标记致盲击杀

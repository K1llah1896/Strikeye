# Spec Delta

## Purpose

把每一次击杀放回地图几何里，回答"隔了多远、中间有没有墙、有没有烟"，让击杀表从事件列表变成可复核的交火记录。

## ADDED Requirements

### Requirement: Per-kill geometry annotation

系统 SHALL 为回放缓存中的每次击杀计算几何上下文：

- 交火距离 SHALL 按击杀者眼位（位置 + 64）到受害者胸口（位置 + 45）的欧氏距离计算。
- 在提供地图几何时，系统 SHALL 用 BVH 判定该连线是否被遮挡，并输出 `visible`。
- 在缺少地图几何时，系统 SHALL 输出 `geometry_checked=false`，距离与烟雾判定仍然可用，且不得报错。
- 系统 SHALL 判定连线是否穿过击杀 tick 生效的烟雾球（半径 144 单位，中心抬高 32 单位），输出 `smoke_blocked`。
- 汇总 SHALL 包含平均交火距离、被遮挡击杀占比、穿烟击杀占比。
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

#### Scenario: Kill table shows geometry

- **WHEN** 用户打开区域控制页的击杀区位表
- **THEN** 每行显示交火距离与视线状态（可见 / 遮挡 / 遮挡·烟 / 未知）

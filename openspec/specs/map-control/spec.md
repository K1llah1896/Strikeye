# map-control Specification

## Purpose
定义基于导航网格可达性的地图控制模型，输出可跨图比较的控制净值曲线与区域级控制摘要，回答"这张图是谁的、什么时候丢的"。

## Requirements

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

### Requirement: Per-callout control summary

系统 SHALL 按 callout 聚合控制结果：

- 每个 callout 输出采样数、平均净值、最小净值、最大净值。
- 区域归属到 callout 使用 callout 数据集对区域质心的归属。
- 缺失 callout 数据时仍输出整图曲线，只是没有区域摘要。

#### Scenario: Callout summary ranks decisive areas

- **WHEN** 一张图的 A 大长期由 CT 控制、B 洞长期由 T 控制
- **THEN** 两者的平均净值分别接近 +1 与 -1，并在按偏离中立程度排序时排在最前

### Requirement: Map control CLI and UI

系统 SHALL 提供控制曲线的命令行与界面出口：

- `strikeye-cli mapcontrol [--demo-id <id>] [--step <sec>] [--radius <units>]` 输出 `map`、`config`、`frames`、`callouts` 的 JSON。
- 导航网格未缓存时命令 SHALL 提示 `navs-fetch`，区域控制页 SHALL 显示同样的指引而不是空白卡片。
- 区域控制页 SHALL 显示净值 sparkline、回合内平均净值、CT 占优时段占比，以及按偏离中立程度排序的区域条形图。

#### Scenario: Missing navmesh degrades gracefully

- **WHEN** 用户打开区域控制页但未缓存 `.nav`
- **THEN** 站位与击杀表照常显示，控制卡片显示 `navs-fetch` 提示

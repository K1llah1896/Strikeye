# kill-context Specification

## Purpose
把每一次击杀放回地图几何里，回答"隔了多远、中间有没有墙、有没有烟"，让击杀表从事件列表变成可复核的交火记录。

## Requirements

### Requirement: Per-kill geometry annotation

系统 SHALL 为回放缓存中的每次击杀计算几何与光学上下文：

- 交火距离 SHALL 按击杀者眼位（位置 + 64）到受害者胸口（位置 + 45）的欧氏距离计算。
- 在提供地图几何时，系统 SHALL 用 BVH 判定该连线是否被遮挡，并输出 `visible`。
- 在缺少地图几何时，系统 SHALL 输出 `geometry_checked=false`，距离与烟雾判定仍然可用，且不得报错。
- 系统 SHALL 判定连线是否穿过击杀 tick 生效的烟雾球（半径 144 单位，中心抬高 32 单位），输出 `smoke_blocked`。
- 汇总 SHALL 包含平均交火距离、被遮挡击杀占比与穿烟击杀占比。
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

### Requirement: Per-kill optics annotation

系统 SHALL 用击杀 tick 之前最近的回放帧判断这一枪的光学条件：

- `aim_off_deg` SHALL 为击杀者视线（yaw）与"击杀者→受害者"连线的夹角，按 XY 平面计算。
- `in_fov` SHALL 为夹角是否落在半视角（默认 53°）内。
- `victim_sees_killer` SHALL 为受害者视线是否指向击杀者。
- `attacker_blind` / `victim_blind` SHALL 由剩余致盲时间大于 0.1 秒判定。
- 没有可用回放帧时，系统 SHALL 输出 `optics_checked=false`，不得把缺失当成"看不见"。
- 汇总 SHALL 包含 `optics_used`、视野内占比、背身击杀占比与致盲击杀数。

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

### Requirement: Stored kill context rows

击杀上下文 SHALL 可持久化并查询：

- 迁移 v12 SHALL 创建 `kill_context` 表，主键 `(demo_id, round_number, tick, victim_slot)`，并建立 `(demo_id, round_number)` 与 `(demo_id, smoke_blocked)` 索引。
- `killctx --store` SHALL 在单事务内重写该 demo 的全部行（空结果即清空）。
- 每行 SHALL 记录武器（按同槽位 ±64 tick 内最近一发推断）、爆头、交火距离、视线与几何/光学检查标记、穿烟、开镜夹角、是否在视锥内、受害者是否看到击杀者、双方致盲与开镜等级、双方世界坐标。
- `Totals` SHALL 在 SQL 中聚合击杀数、平均距离、背身率与穿烟率。
- 比赛导出 SHALL 包含击杀行：CSV `match_<id>_kills.csv`、XLSX `kills` 表、JSON `kills` 数组。

#### Scenario: Store then query

- **WHEN** 用户对一场 demo 运行 `killctx --store`，然后执行 `strikeye-cli sql --query "select count(*) from kill_context"`
- **THEN** 返回该场的击杀条数，且每行都有武器与坐标

#### Scenario: Export includes kills

- **WHEN** 已落库的 demo 导出 CSV
- **THEN** `match_<id>_kills.csv` 出现，行数与库内一致，`distance` 保留两位小数

#### Scenario: Re-store replaces

- **WHEN** 同一 demo 再跑一次 `--store`
- **THEN** 表中仍只有一份数据（不重复累积）

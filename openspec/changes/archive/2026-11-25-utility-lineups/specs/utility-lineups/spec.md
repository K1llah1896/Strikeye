# Spec Delta

## Purpose

把逐颗投掷物聚合成可复用的道具线位，输出类型、侧别、投掷点、落点、使用次数、覆盖回合与后续击杀，供教练与选手直接对照战术板。

## ADDED Requirements

### Requirement: Lineup clustering

系统 SHALL 把同方、同类型、出手点与落点相近的投掷聚合成一条线位：

- 出手点与落点按 128 世界单位网格取整；类型与侧别参与聚类键。
- 每条线位输出使用次数、覆盖回合数、平均出手时间（相对 freeze time）、平均出手点与平均落点。
- 侧别由 `match_players.team_key` 与回合的 `team_a_side`/`team_b_side` 解析，中场换边后不得混侧。
- 无轨迹点的投掷计入投掷总数但不参与聚类。

#### Scenario: Two identical smoke throws merge

- **WHEN** 同一方在同一区域扔出两颗落点相近的烟雾弹
- **THEN** 它们合并为一条线位，使用次数为 2，覆盖回合数为去重后的回合数

#### Scenario: Same landing spot on opposite sides stays separate

- **WHEN** T 与 CT 在同一落点各扔一颗烟雾弹
- **THEN** 输出两条线位，侧别分别为 `t` 与 `ct`

### Requirement: Follow-up effect

系统 SHALL 为每条线位计算后续击杀：

- 统计落地 tick 之后 8 秒内、落点 600 世界单位内的击杀数。
- 排序以使用次数为第一关键字、后续击杀为第二关键字。

#### Scenario: Smoke that sets up a kill

- **WHEN** 一颗烟雾弹落地 2 秒后落点附近出现一次击杀
- **THEN** 该线位的后续击杀数为 1

#### Scenario: Kill far from the landing spot is ignored

- **WHEN** 落地后 2 秒内发生的击杀距落点超过 600 单位
- **THEN** 该击杀不计入线位

### Requirement: Lineup naming and reporting

系统 SHALL 用 callout 数据集给线位命名，并提供命令行与界面出口：

- 投掷点与落点用聚类中心的平均坐标查询 callout；缺失数据时显示"未命名"。
- `strikeye-cli nades [--demo-id <id>] [--min-throws <n>]` 输出 `map`、`total_throws`、`named_throws`、`lineups`、`count` 的 JSON。
- Match Analysis 的区域控制页 SHALL 显示按使用次数排序的前 12 条线位，包含道具、侧别、投掷点 → 落点、次数、回合、后续击杀与平均出手时间。

#### Scenario: Cluster named by its centre

- **WHEN** 一条线位的平均出手点落在 B 门、但有单次投掷在 A 小
- **THEN** 线位的投掷点显示 B 门

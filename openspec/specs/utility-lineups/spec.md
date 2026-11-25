# utility-lineups Specification

## Purpose
把逐颗投掷物聚合成可复用的道具线位，输出类型、侧别、投掷点、落点、使用次数、覆盖回合与后续击杀，供教练与选手直接对照战术板。

## Requirements

### Requirement: Lineup clustering

系统 SHALL 按同侧、同道具类型、投掷点与落点网格（128 单位）聚类投掷，并满足：

- 聚类 SHALL 记录使用回合数（去重）、获胜回合数（去重）、回合胜率。
- 系统 SHALL 计算 `impact = 该线位回合胜率 − 同队同侧基准胜率`，单位为百分点，保留一位小数。
- 基准胜率 SHALL 只用该队在该侧打的回合计算；缺少胜者信息的回合 SHALL 计入分母但不计入分子。
- 系统 SHALL 输出 `kills_per_throw`，为落点附近 8 秒/600 单位内击杀数与该线位投掷次数之比。
- 线位列表 SHALL 先按使用次数降序，次数相同时按 `impact` 降序。

#### Scenario: Line-up beats the team baseline

- **WHEN** 某条烟在 3 个回合被使用、赢下其中 2 个，而该队该侧 4 个回合赢 2 个
- **THEN** `win_rate` 为 66.7，`impact` 为 +16.7

#### Scenario: Two identical smoke throws merge

- **WHEN** 同一方在同一区域扔出两颗落点相近的烟雾弹
- **THEN** 它们合并为一条线位，使用次数为 2，覆盖回合数为去重后的回合数

#### Scenario: Same landing spot on opposite sides stays separate

- **WHEN** T 与 CT 在同一落点各扔一颗烟雾弹
- **THEN** 输出两条线位，侧别分别为 `t` 与 `ct`

#### Scenario: Missing winner data

- **WHEN** 回合没有记录获胜方
- **THEN** 该回合计入线位的回合数，但不计入获胜数，也不计入基准胜率的分子

#### Scenario: Smoke that sets up a kill

- **WHEN** 一颗烟雾弹落地 2 秒后落点附近出现一次击杀
- **THEN** 该线位的后续击杀数为 1

#### Scenario: Table shows impact

- **WHEN** 用户查看道具线位表
- **THEN** 每行显示回合胜率与收益，收益绝对值不小于 10 个百分点时着色

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

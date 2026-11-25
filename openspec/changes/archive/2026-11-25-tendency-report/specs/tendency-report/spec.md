# Spec Delta

## Purpose

把一个队伍的历史比赛聚合成赛前侦察报告：分图分侧战绩、手枪局、经济类型战绩、T 侧下包点分布与开场击杀区域，供教练在赛前制定针对性战术，无需逐场翻 demo。

## ADDED Requirements

### Requirement: Team scouting profile

系统 SHALL 按队伍聚合已解析比赛，产出侦察报告：

- SHALL 只统计 `parse_status = parsed` 且队伍名匹配 `team_a`/`team_b` 的 demo。
- 分图分侧 SHALL 给出回合数、胜场、胜率、手枪局（第 1 与第 13 回合）胜场、平均装备值与每回合道具量。
- 经济维度 SHALL 按侧别 × 经济类型给出回合数与胜率，类型为空时归入 `unknown`。
- 下包点分布 SHALL 只统计该队处于 T 侧的回合，按 `bomb_site` 汇总下包次数与转化率。
- 开场击杀 SHALL 取每回合最早一次死亡，击杀者属于该队时计入，并按死者位置映射 callout、记录 `freeze_end_tick` 起算的平均秒数。
- 缺少 callout 数据时 SHALL 仍产出其余字段，并把 `callouts_used` 置为 false。
- 队伍名为空 SHALL 返回错误；无匹配 demo SHALL 由调用方判为零回合。

#### Scenario: Side split and economy

- **WHEN** 某队在两回合分别处于 CT（手枪局，4000 经济，胜）与 CT（force，9000 经济，负）
- **THEN** CT profile 为 `rounds=2, wins=1, pistol_wins=1, avg_equip=6500`

#### Scenario: Site preference only counts own T rounds

- **WHEN** 该队 T 侧两回合分别在 B 与 A 下包，另有对手 T 侧回合下包
- **THEN** `sites` 只含 B 与 A 各 1 次下包，且不含对手那一次

#### Scenario: Utility per round filters by roster

- **WHEN** 该队选手在某回合买了 1 颗烟，对手买了 1 颗烟
- **THEN** 该回合本方道具计数为 1

#### Scenario: Opening duel attribution

- **WHEN** 某回合最早死亡由该队选手击杀，且 `freeze_end_tick=600`、死亡 tick=1000、tick rate 64
- **THEN** 该回合计入一次开场击杀，耗时 6.25 秒

### Requirement: Tendency CLI

系统 SHALL 提供 `strikeye-cli tendencies` 输出侦察报告：

- `--team` SHALL 必填；缺失时返回错误。
- `--format json|csv` SHALL 支持两种输出，CSV 用 `kind` 列区分 side/economy/site/opening。
- `--top` SHALL 限制开场击杀 callout 条数（默认 8）。
- `--out` SHALL 写入文件而非 stdout。
- 缺 callout 数据时 SHALL 在 stderr 提示运行 `strikeye-cli callouts-fetch`。

#### Scenario: CSV output shape

- **WHEN** 报告含 1 个侧别、1 个经济类型、1 个下包点、1 个开场区域
- **THEN** CSV 为表头 + 4 行，且侧别行含 `equip=`、`utility_per_round=`、`pistols=` 附加字段

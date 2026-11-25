# Spec Delta

## Purpose

补充赛前面向"对阵"与"个人走势"的两个视图：两队历史交手的分图战绩，以及单个选手逐场表现的时间序列。

## ADDED Requirements

### Requirement: Head-to-head record

系统 SHALL 汇总两支队伍之间的历史交手：

- 只统计 `parse_status = parsed` 且两个队名与 demo 的 `team_a`/`team_b` 匹配（忽略大小写）的比赛。
- 比分 SHALL 归一化到调用方给出的第一支队（`wins_a`/`rounds_a`）。
- SHALL 输出总体场次、双方回合数与胜场、按地图分组的场次/胜场/回合数。
- SHALL 输出最近交手列表，按 `match_date` 降序、同日期按 demo id 降序。
- 队名为空或两队同名 SHALL 返回错误。

#### Scenario: Orientation is normalised

- **WHEN** demo 存为 `team_a=Falcons, team_b=FURIA`，比分 6-13，调用 `h2h --team-a FURIA --team-b Falcons`
- **THEN** `wins_a` 加一且 `rounds_a` 增加 13

#### Scenario: Per-map aggregation

- **WHEN** 两队在两场 de_dust2 上各赢一场（13-6 与 9-13）
- **THEN** de_dust2 记录为 `matches=2, wins_a=1, wins_b=1, rounds_a=22, rounds_b=19`

#### Scenario: Unparsed demos are ignored

- **WHEN** 两队还有一场 `parse_status = pending` 的比赛
- **THEN** 该场不出现在 `matches`/`maps`/`recent` 中

### Requirement: Player match trend

系统 SHALL 输出单个选手逐场比赛的时间序列：

- SHALL 按 `match_date` 升序、同日期按 demo id 升序排列。
- 每场 SHALL 含 demo id、地图、日期、对手队名、选手所在 `team_key`、回合数、K/D/A、ADR、KAST、HS%、Rating 与胜负。
- 对手队名 SHALL 由选手的 `team_key` 反查 `team_a`/`team_b` 得到；胜负 SHALL 用该队比分比较。
- 空 steamid SHALL 返回错误；无比赛 SHALL 返回空切片而不是错误。

#### Scenario: Opponent and result

- **WHEN** 选手在 `team_b`，demo 为 `team_a=Falcons, team_b=FURIA`，比分 14-16
- **THEN** 该场 `opponent=Falcons`、`won=true`

#### Scenario: Chronological order

- **WHEN** 选手分别有 2026-11-03 与 2026-11-01 两场
- **THEN** 输出顺序为 11-01 在前

### Requirement: Head-to-head and trend CLI

系统 SHALL 提供 `strikeye-cli h2h` 与 `strikeye-cli trends`：

- `h2h` SHALL 接受 `--team-a`、`--team-b`、`--format json|csv`；CSV 首行为 overall 行。
- `trends` SHALL 接受 `--steamid`、`--format json|csv`；无比赛时返回错误并提示 steamid。
- 两者缺少必填参数 SHALL 返回错误而不是 panic。

#### Scenario: Missing arguments

- **WHEN** 运行 `trends` 未提供 `--steamid`
- **THEN** 命令返回错误

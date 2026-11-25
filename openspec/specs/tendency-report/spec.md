# tendency-report Specification

## Purpose
把一个队伍的历史比赛聚合成赛前侦察报告：分图分侧战绩、手枪局、经济类型战绩、T 侧下包点分布与开场击杀区域，供教练在赛前制定针对性战术，无需逐场翻 demo。

## Requirements

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

### Requirement: Player trend sparkline in the analysis drawer

系统 SHALL 在展开的选手抽屉里显示跨场走势：

- 选手指最近比赛多于一场时 SHALL 显示一行，含 Rating sparkline、首尾 Rating 差值、窗口内胜场数、ADR sparkline 与首尾 ADR 差值。
- SHALL 追加最近三场的地图、对手、胜负与 Rating 摘要。
- SHALL NOT 在渲染路径同步查询数据库；数据 SHALL 在展开时后台加载并按 SteamID64 缓存。
- 选手仅有单场比赛或数据缺失时 SHALL NOT 显示该行。
- 对手名缺失时 SHALL 显示"未知对手"，地图缺失时显示 `?`。

#### Scenario: Sparkline window

- **WHEN** 选手有 3 场比赛（Rating 1.10 → 1.40 → 0.90，其中 2 胜）
- **THEN** 行内为 `近 3 场 Rating <sparkline> 1.10 → 0.90 (2 胜)` 与 `ADR <sparkline> 90.0 → 70.0`

#### Scenario: Window trimming

- **WHEN** 以窗口 2 渲染三场数据
- **THEN** 展示最近两场，首值为 1.40

#### Scenario: Single match

- **WHEN** 选手只有一场已解析比赛
- **THEN** 不渲染走势行

#### Scenario: Unknown opponent

- **WHEN** 某场数据的对手队名为空
- **THEN** 摘要中以"未知对手"占位而不是空串

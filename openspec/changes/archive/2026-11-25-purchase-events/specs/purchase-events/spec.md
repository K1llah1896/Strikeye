# purchase-events

## ADDED Requirements

### Requirement: Buy-window purchase capture

解析器 SHALL 在买枪窗口内把每笔购买记为一条 `PlayerPurchase`：

- 窗口 SHALL 为回合开始 tick 到冻结期结束 tick；窗口外的记录 SHALL 被丢弃。
- 出生免费装备（`Knife`、`C4`、`Glock-18`、`USP-S`、`P2000`）与刀皮名 SHALL NOT 计入购买。
- 只有 `WeaponPrice` 已知的物品 SHALL 被记录；未知物品 SHALL 跳过而不是记 0 元。
- 每条记录 SHALL 带 `steam_id64`、`player_name`、`weapon`、`cost`、`tick`。
- `time_sec` SHALL 相对冻结期结束计算，冻结期内的购买为负值。

#### Scenario: Record a rifle buy

- **WHEN** 回合开始后玩家在冻结期内购买 AK-47
- **THEN** 记录一条 cost=2700 的购买，`time_sec` 为负

#### Scenario: Ignore spawn loadout

- **WHEN** 玩家在回合开始时拿到 Knife 与 Glock-18
- **THEN** 不产生任何购买记录

#### Scenario: Ignore post-freeze pickup

- **WHEN** 冻结期结束后玩家捡起地上的 AWP
- **THEN** 不产生购买记录

#### Scenario: Ignore purchase before round start

- **WHEN** 上一个回合结束后、当前回合开始前发生拾取
- **THEN** 不记入当前回合

### Requirement: Purchase persistence

系统 SHALL 在解析结束时整场替换 `match_purchases`：

- 同一 demo 重复解析 SHALL NOT 产生重复行。
- 列表 SHALL 按 `round_number`、`tick` 排序。
- 汇总 SHALL 能按选手给出花费合计。

#### Scenario: Re-parse replaces rows

- **WHEN** 同一 demo 解析两次
- **THEN** 行数为第二次解析的购买数，而不是两次之和

#### Scenario: Totals per player

- **WHEN** 查询某 demo 的购买汇总
- **THEN** 每位选手的金额等于其所有购买 `cost` 之和

### Requirement: Purchase reporting

`strikeye-cli buys` SHALL 输出按回合聚合的购买明细，并支持 `--round` 过滤：

- 输出 SHALL 含 `demo_id`、`purchases`、`total_spend`、`avg_per_round` 与 `rounds[]`。
- 每个回合 SHALL 给出 `total_spend` 与按花费降序的选手列表，选手下含逐笔 `items`。
- 无购买时 `rounds` SHALL 序列化为 `[]`。

#### Scenario: Round filter

- **WHEN** 以 `--round 2` 调用
- **THEN** 只返回第 2 回合，`avg_per_round` 等于该回合总额

#### Scenario: Empty demo

- **WHEN** demo 没有购买记录
- **THEN** 输出 `purchases: 0` 且 `rounds` 为 `[]`

### Requirement: Round drawer purchase detail

比赛分析的回合详情抽屉 SHALL 在每个选手的经济行下追加该选手本回合的购买：

- 每行 SHALL 显示花费合计与逐笔"物品 + 时刻"。
- 时刻 SHALL 以 `mm:ss` 表示，冻结期内的购买带负号。
- 抽屉 SHALL 给出本回合购买笔数与合计金额。

#### Scenario: Player with purchases

- **WHEN** 展开一个选手有购买记录的回合
- **THEN** 该选手行下出现「购买 $2900 · AK-47 -0:08 · 闪光 -0:05」

#### Scenario: Player without purchases

- **WHEN** 选手本回合没有购买记录
- **THEN** 不渲染该选手的购买行

# Proposal: 对手倾向报告（anti-strat profile）

## Why

调研里 `Twoos123/cs2-meta-engine` 的核心卖点是"反针对报告"：把一个队的回合切分成 CT 默认站位、T 默认展开、手枪局、购买类型、进攻点位，让教练在赛前十分钟就知道对手的套路。Strikeye 已经把这些原料（回合侧别、经济类型、装备值、炸弹点、逐件购买、开场击杀、callout 分区）全部落库，但没有任何一个视图把它们按"某个队"汇总起来——`team` 只给地图战绩和阵容，分析师仍要手算。

## What Changes

- 新增 `internal/tendency`：按队伍聚合出 scout 报告（分图分侧战绩、手枪局、平均装备、每回合道具量、购买类型战绩、T 侧下包点分布、开场击杀所在 callout 与平均耗时）。
- 新增 `strikeye-cli tendencies --team <name>`：JSON 或 CSV（CSV 用一个 `kind` 列把四类事实拉平，方便透视表）。
- 只读：不新增表，纯查询 + 回放解码；缺 callout 数据时报告仍产出，只是开场分区为空并给出提示。

## Impact

- Affected specs: `tendency-report`（新增）
- Affected code: `internal/tendency`、`cmd/strikeye-cli/tendency.go`、`main.go` 子命令与 usage

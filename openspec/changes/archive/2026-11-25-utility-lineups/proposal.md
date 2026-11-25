# Change: Utility Lineup Clustering

## Why

调研报告把"手雷 line-up 库"列为 cs-demo-manager 与 cs2-meta-engine 的核心差异化能力：前者能按地图筛选职业队常用道具，后者把 line-up 按影响力排序。Strikeye 目前只有投掷物轨迹的 2D 图层，没有任何"这套道具是什么、从哪扔、扔到哪、效果如何"的聚合视图，教练与选手仍然只能逐回合拖时间轴。

回放缓存里已经有全部原料：每颗手雷的投掷者、类型、出手 tick 与完整轨迹点，死亡事件带位置与 tick，callout 数据集给出世界坐标到地名的映射。缺的是把"逐颗投掷"聚合成"线位"的模型。

## What Changes

- 新增 `internal/nadebook`：把同方、同类型、出手点与落点相近（默认 128 世界单位网格）的投掷聚合成一条 line-up，输出使用次数、覆盖回合、平均出手时间、平均落点，以及"后续击杀"（落点 600 单位内、落地后 8 秒内的击杀数）作为效果代理指标。
- 侧别按 `match_players.team_key`（`team_a`/`team_b`）结合回合的 `team_a_side`/`team_b_side` 解析，中场换边后同一名选手的投掷不会被混到错误一方。
- 线位用聚类中心的平均坐标去查 callout，避免整条线位被单次偏心投掷命名错。
- CLI 新增 `nades --demo-id --min-throws`，输出 JSON（类型/侧/投掷点/落点/次数/回合/后续击杀/平均时间）。
- Match Analysis 的区域控制页新增「道具线位」卡片，按使用次数列前 12 条，显示投掷点 → 落点、次数、回合、后续击杀与平均出手时间。
- 补充单测：聚类合并、侧别解析（含中场换边）、无轨迹/无 callout 退化、类型别名归一。

## Impact

- Affected specs: `utility-lineups`（新增）
- Affected code: `internal/nadebook`、`cmd/strikeye-cli`、`internal/ui/views/analysis`
- 性能：dust2（442 次投掷）与 ancient（785 次投掷）单次聚合约 10ms，可直接在标签页后台计算。

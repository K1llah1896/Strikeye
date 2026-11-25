# Design: Utility Lineup Clustering

## 聚类键

一条 line-up 由 `(类型, 侧别, 出手网格, 落点网格)` 决定：

- 类型先归一（`CSmokeGrenadeProjectile` → `smoke`，`CMolotovProjectile` → `molotov`，`CIncendiaryGrenade` → `incendiary`）。
- 出手点与落点按 128 世界单位取整到网格。网格太小会把同一套道具拆成多条，太大又会把不同烟位合并；128 单位约等于一个烟雾半径的 1.8 倍，实测 dust2 442 次投掷聚成 16 条 ≥3 次的线位。
- 侧别必须参与键：T 的进攻烟与 CT 的防守烟即便落点相同，也是两套完全不同的战术意图。

## 侧别解析

回放的 `GrenadeTrack.ThrowerSteamID` 只给到选手，`match_players.team_key` 存的是 `team_a`/`team_b`，而每回合的阵营在 `match_rounds.team_a_side`/`team_b_side`：

1. `team_a` → 回合的 `team_a_side`；`team_b` → `team_b_side`。
2. 其他键名回退到与 demo 的 `team_a_name` 比较，仍不匹配则视为 B 队。
3. 空键不参与侧别，避免把未知选手的投掷算到某一方。

这样中场换边后，同一选手在上半场的 `team_a/T` 与下半场的 `team_a/CT` 会被正确区分。

## 效果指标

调研里 cs-demo-manager 的"影响力"排序需要回合结果与击杀归因；本仓库没有逐颗手雷的伤害归因，因此用"后续击杀"作代理：

- 统计落地 tick 之后 8 秒内、落点 600 单位内的击杀。
- 这是一个下界：封烟逼退、闪光致盲造成的收益不算击杀，所以排序把使用次数放第一位、后续击杀放第二位，而不是只按击杀排。

## 命名

线位名用聚类中心的平均坐标查 callout（`Set.At` 支持精确盒与 512 单位邻近回退），而不是取"第一条投掷"的位置，否则一条 8 次投掷的烟位可能因为一次偏心投掷被叫成隔壁区域。

## 退化

- 无轨迹点的投掷计入总数但不聚类。
- 缺 callout 数据时照常聚类，只是投掷点/落点显示"未命名"。
- 无回放缓存时卡片显示空状态，不阻塞其它区域卡片。

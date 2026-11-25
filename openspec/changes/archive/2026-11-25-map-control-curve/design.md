# Design: Map Control Curve

## Context

控制（control）没有唯一定义。可选模型：

1. **raycast**：区域内任一点能否被某队看到——需要逐区域视线采样，成本高，且"看得见"不等于"控得住"。
2. **reachability**：某队从当前位置能在多久内走到该区域——只需一次截断 Dijkstra，对烟雾/火焰天然友好（把受阻区域从图里摘掉即可）。
3. **vision + reachability 加权**：awpy 的完整模型，参数多、调参难。

## Decisions

### 先做 reachability

Strikeye 的目标是"讲清这回合为什么输"，reachability 直接对应"轮转速度/包点压力"这类教练语言；raycast 留给后续在 nav 上叠加烟雾几何时再补。截断半径 1100 单位 ≈ 4.5 秒跑动（250 u/s），正好是 CS2 一次轮转的典型时间尺度。

### 净值而非绝对区域数

不同地图 nav area 密度差一倍以上（dust2 2242 / ancient 1969），绝对值不可跨图比较。净值 `(CT - T) / 已分类区域` 落在 [-1, 1]，跨图可比、可画曲线。争夺区域不计入分子分母。

### 争判定用距离差而不是布尔可见

`|dT - dCT| < 200` 判争夺：两队几乎同时能到，说明这是"打起来的地方"。200 单位 ≈ 0.8 秒跑动，小于一次 peek 的时间，避免把整张图都算成争夺。

### 时间轴从 freeze time 起算

回放索引的 `StartTick` 含热身（实测 round 1 的 freeze 在 88 秒后），若不校正，曲线前段会被出生点静止数据压平。`match_rounds.freeze_end_tick` 是权威基准。

### 每 callout 聚合，而不是每 area

area 级输出 2000+ 行对用户没有意义。按 callout 聚合后只剩 30–60 行，且与用户心智模型（"A 大控制"）一致。排序按 `|avg_net|`，直接给出"决定这张图的区域"。

## Risks

- 只考虑地面连通性，不考虑高低差与跳跃（`.nav` 的 ladder/jump 连接未参与代价），对 nuke/vertigo 这类立体图会偏乐观。
- 未接入烟雾/火焰遮挡：目前控制曲线是"无道具干扰"的基线，后续可在 Dijkstra 中屏蔽 smoke/inferno 覆盖的 area。

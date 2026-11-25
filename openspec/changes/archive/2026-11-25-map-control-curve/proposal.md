# Change: Map Control Curve

## Why

上一轮把 navmesh 与 callout 数据接进来后，Strikeye 能说"谁站在哪个区域"，但还答不了"这块地图是谁的、什么时候丢的"。这是调研报告里唯一完全没有的队伍级分析维度，也是 awpy 与 cs2-meta-engine 的差异化能力：

- awpy 的 `map_control` 按 nav area 输出 CT/T/contested/neutral 与净值曲线；
- cs2-meta-engine 用 CT 默认站位与 T 默认展开（0:20）讲同一件事。

Strikeye 已有全部原料（`.nav` 导航网格、碰撞几何、32Hz 回放帧、callout 表），缺的是控制归属模型。

## What Changes

- 新增 `internal/mapcontrol`：以"可达性"为控制模型——对每支队伍从存活选手所在区域做截断 Dijkstra（默认半径 1100 单位），两队的到达距离差小于 200 单位判为争夺，输出每帧的 T/CT/争夺/中立区域数与净值。
- 按 callout 聚合净值，输出平均/最小/最大控制，用于回答"哪个区域决定了这张图"。
- 采样从 freeze time 开始、默认每 5 秒一次，避免热身时间污染曲线。
- CLI 新增 `mapcontrol --demo-id --step --radius`。
- Match Analysis 的区域控制页新增「地图控制曲线（navmesh 可达性模型）」卡片：净值 sparkline、CT 占优时段占比、按偏离中立程度排序的区域条形图；未缓存导航网格时提示 `navs-fetch`。

## Impact

- Affected specs: `map-control`（新增）
- Affected code: `internal/mapcontrol`、`cmd/strikeye-cli`、`internal/ui/views/analysis`
- 性能：dust2（2242 区域 / 19 回合 / 409 采样）CLI 全量计算 0.6 秒，可在标签页后台加载。

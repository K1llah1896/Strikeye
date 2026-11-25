# 队伍视角标签页

## 为什么

队伍层能力（`team` 档案、`tendencies` 侦察报告、`h2h` 交手矩阵、`series` 系列赛）此前只有 CLI。桌面应用是主要使用界面，分析师打开一场 demo 时看不到"这支队在别的图上怎么打、对手是谁、历史交手如何"——报告 §4.6 写的是 ✔，实际只有命令行可达，属于能力与界面不一致。

## 变什么

1. Match Analysis 新增第 8 个标签页「队伍视角」，用当前 demo 的两支队伍名做切换：
   - 队伍档案：地图胜-负、回合胜-负、阵容（前 5 人）、分图战绩（比分/对手/回合/Rating）。
   - 侦察报告：分图分侧的胜率/手枪局/平均装备/每回合道具、经济类型战绩、T 侧下包点转化率、开场击杀区位。
   - 交手记录：当前两队的大场比分、分图战绩、最近 5 场。
2. 回放缓存与 callout 数据按需加载；缺失时对应区块降级（缺 callout 时提示 `callouts-fetch`），不阻塞整页。
3. `internal/parser` 抽出 `ResolveReplayCachePath` / `LoadReplayCache` / `ReplayRoundList`，CLI 的 `zones.go` 改为委托，避免两份缓存路径回退逻辑各自演化。

## 影响

- capability：`match-library-and-analysis-ui`（ADDED「Team workspace tab」）。
- 依赖既有 `team`、`tendency`、`h2h` 包，无新数据表。

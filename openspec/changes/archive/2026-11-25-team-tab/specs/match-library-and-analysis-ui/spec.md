# match-library-and-analysis-ui

## ADDED Requirements

### Requirement: Team workspace tab

Match Analysis SHALL 提供队伍视角标签页，把跨场次队伍数据带到 GUI：

- 标签页 SHALL 用当前 demo 的 `team_a` / `team_b` 名称作为可切换对象，并允许在同一场里切换看哪一支队。
- 队伍档案区块 SHALL 显示地图胜-负、回合胜-负、阵容（最多 5 人，含图数/回合/击杀-死亡/Rating）与分图战绩（比分、对手、回合数、Rating）。
- 侦察报告区块 SHALL 显示分图分侧的战绩（胜率、手枪局、平均装备、每回合道具）、经济类型战绩、T 侧下包点转化率、开场击杀区位（最多 8 项）。
- 交手记录区块 SHALL 显示当前两队的图数胜负、回合小分与最近 5 场比分。
- 缺少 callout 数据时 SHALL 在开场击杀区块提示运行 `callouts-fetch`，其余区块照常显示。
- 缺少回放缓存时 SHALL 跳过侦察报告中依赖回放的部分，不得让整页失败。
- 队伍没有任何已解析比赛时 SHALL 显示引导文案而不是错误。
- 数据加载 SHALL 在后台执行，且同一 (队伍, 对手) 组合只加载一次。

#### Scenario: Switch focus team

- **WHEN** 用户在队伍视角点击另一支队的标签
- **THEN** 档案、侦察报告与交手记录都切换为该队视角，交手对手自动变成当前 demo 的另一支队伍

#### Scenario: Missing callouts

- **WHEN** `cache/map_data/callouts.csv` 不存在
- **THEN** 开场击杀区块显示 `callouts-fetch` 提示，档案与分侧表仍正常显示

#### Scenario: Unknown team name

- **WHEN** 队伍在资料库里没有已解析比赛
- **THEN** 页面显示"导入并解析该队的 demo 后…"的引导文案，不抛错

### Requirement: Replay cache resolution helper

`internal/parser` SHALL 提供统一的回放缓存定位与读取入口：

- `ResolveReplayCachePath(dataRoot, stored)` SHALL 优先返回存储路径，文件不存在时回退到 `cache/replay_32hz/<文件名>`。
- `LoadReplayCache(dataRoot, demo)` SHALL 返回每回合的回放数据；没有可用缓存时 SHALL 返回空 map 与 nil error。
- CLI 与 UI SHALL 共用该实现，不得各自维护路径回退逻辑。

#### Scenario: Moved DataRoot

- **WHEN** demo 行里记录的缓存路径已不存在，但同名文件在新 DataRoot 下存在
- **THEN** 读取成功并返回每回合数据

#### Scenario: No cache

- **WHEN** demo 从未生成回放缓存
- **THEN** 返回空 map 且无错误，调用方按"无空间数据"处理

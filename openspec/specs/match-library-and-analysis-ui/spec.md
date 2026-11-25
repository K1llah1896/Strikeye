# match-library-and-analysis-ui Specification

## Purpose

Defines the Match Library browser, bundled map radar assets, and the five-tab Match Analysis workspace including scoreboard, economy curves, CS Data Radar workbench, 32Hz 2D Replay, and KDE Heatmaps.

## Requirements

### Requirement: Match Library search, filtering, and quick actions
The `Match Library` view SHALL query SQLite WAL for ingested demos and support filtering by keyword (map, player name, note, filename), map, platform, parse/render status, and favorites, displaying each match card anchored on prominent team names and scoreline, map/platform/status capsules, and clearly prioritized quick-navigation actions. Keyword matching SHALL be extended by full-text index hits so notes stored outside the card model stay searchable.

#### Scenario: Filter and open match analysis
- **WHEN** the user enters a map or player keyword and clicks "Match Analysis" on a parsed match card
- **THEN** the list filters to matching demos and navigates directly to `Match Analysis` for the selected demo

#### Scenario: Full-text hits extend the keyword match
- **WHEN** the keyword matches nothing in memory but the FTS index reports the demo id (for example a tactical annotation note)
- **THEN** the demo is still shown

#### Scenario: Index refresh is asynchronous
- **WHEN** the keyword changes while a previous FTS query is in flight
- **THEN** no second query is issued for that keyword until the first finishes, and the render loop never blocks on SQLite

### Requirement: Manual demo import and watch-directory configuration
The `Match Library` toolbar SHALL allow users to manually add a local `.dem` file (triggering `demo.parse` over IPC) and configure watched demo directories.

#### Scenario: Manual demo addition
- **WHEN** the user selects a valid `.dem` file via "Add Demo"
- **THEN** the UI enqueues the file for Single-Pass parsing via IPC and displays its `parsing` status capsule until ready

### Requirement: Overview scoreboard and rounds/economy analysis tabs
The `Match Analysis` view SHALL provide an `Overview & Scoreboard` tab with MVP/highlight leader cards and a CT/T faction-grouped 10-player table with aligned columns sortable by K, D, A, K/D, ADR, KAST%, HS%, FK, FD, TK, Multi-Kills, Clutches, and `Strikeye Rating`, plus a `Rounds & Economy` tab plotting round-by-round win sequence badges and proportional team equipment-value bars with buy-type annotations.

#### Scenario: Sort scoreboard by Strikeye Rating
- **WHEN** the user clicks the `Strikeye Rating` column header
- **THEN** player rows re-order by Strikeye Rating descending or ascending with aligned numeric columns

### Requirement: CS Data Radar workbench with avatar/logo re-rendering
The `Players & CS Data Radar` tab SHALL support side-by-side comparison of any two players with six-axis and detailed stat comparison, batch generation of `2560x1440` radar cards and `4.0s` intro MP4s via `internal/radar`, custom player avatar and team logo binding with automatic re-render, and in-app preview.

#### Scenario: Re-render radar card on custom avatar update
- **WHEN** the user binds a custom avatar image to a player in the radar workbench
- **THEN** the corresponding `2560x1440` radar card regenerates and updates in the preview component

### Requirement: 32Hz 2D tactical replay data source
The `2D Replay` and `Heatmaps` tabs SHALL read `cache/replay_32hz/<sha256>.rpl` (`CS2RPL01`) and bundled `assets/maps/` radar images plus transform metadata, falling back to a coordinate grid when a map image is missing.

#### Scenario: Play 2D replay without CS2 or video
- **WHEN** the user opens `2D Replay` for any parsed demo and selects a round
- **THEN** the native canvas animates all 10 players and utility effects at 32Hz alongside round/playback controls without requiring any rendered MP4 or launching CS2

#### Scenario: Canvas layers cover combat and utility
- **THEN** the canvas renders player poses, view cones, flash rings, C4 carrier markers, firing rays, grenade arcs, smoke circles, and molotov polygons for the selected round

#### Scenario: Layout keeps controls alongside the canvas
- **THEN** the view uses a balanced split-pane layout with tactical controls, player/team/round-type filters, and a layer legend beside the canvas

#### Scenario: Heatmap modes aggregate replay samples
- **THEN** the `Heatmaps` tab renders KDE position, firefight, kill, and death heatmaps derived from the same replay cache

### Requirement: Localized tactical labels instead of raw database enums
`Match Library` and `Match Analysis` SHALL render human-readable localized labels for parse status (`ready`, `parsing`, `pending`, `failed`), economy buy types (`full_buy`, `half_buy`, `eco`, `force_buy`, `pistol`), and round win reasons (`elimination`, `bomb_exploded`, `bomb_defused`, `time_ran_out`) rather than exposing raw snake_case identifiers.

#### Scenario: Viewing economy and match status badges
- **WHEN** a match with `ready` parse status and `full_buy` / `elimination` rounds is displayed in `Match Library` or `Rounds & Economy`
- **THEN** the UI renders localized tactical labels (e.g., `已解析`, `长枪局`, `全歼`) instead of raw database enum strings

### Requirement: Streamlined Match Library toolbar and centered scoreline anchor
The `Match Library` top toolbar SHALL avoid duplicating watch-directory management inline, and each match card SHALL anchor `Team A  Score : Score  Team B` cleanly with balanced spacing between the scoreline and match remarks.

#### Scenario: Browsing Match Library cards
- **WHEN** the user opens `Match Library`
- **THEN** the toolbar presents a compact filter/import layout and each match card displays a balanced team scoreline without a wide empty gap before the remark

### Requirement: Borderless scoreboard headers, economy column headers, and structured empty states
`Overview & Scoreboard` SHALL use a borderless, aligned table header row with active sort indicators instead of 13 bordered button boxes; `Rounds & Economy` SHALL display `TEAM A (CT)` and `TEAM B (T)` column headers; and `Radar` / `2D Replay` SHALL render structured empty-state guides (including when `replayIndex` is empty) instead of blank dark rectangles.

#### Scenario: Opening 2D Replay without cached replay frames
- **WHEN** the user opens `2D Replay` for a demo that has no `CS2RPL01` replay index
- **THEN** the round selector area displays an explicit empty-state message and the canvas shows a structured tactical placeholder guide

### Requirement: 2D Replay canvas displays map radar backdrop when frames are absent
When `2D Replay` is configured for a demo without `CS2RPL01` replay frames, the `560x560` radar canvas SHALL render the bundled map radar image (or procedural tactical grid fallback) instead of a solid black rectangle.

#### Scenario: Opening 2D Replay without cached replay frames
- **WHEN** the user opens `2D Replay` for a parsed `de_mirage` demo with no replay frames
- **THEN** the `560x560` canvas displays the `de_mirage` tactical radar backdrop

### Requirement: Pixel-aligned scoreboard column headers and rows
The `Overview & Scoreboard` header row and CT/T player data rows SHALL share identical horizontal padding and column widths so every metric header aligns directly above its values.

#### Scenario: Viewing Overview & Scoreboard columns
- **WHEN** the user views `Overview & Scoreboard`
- **THEN** all 13 sortable column headers (`K` through `Rating`) align vertically with the numeric cells below them

### Requirement: Collapsible manual demo import in Match Library
The `Match Library` toolbar SHALL keep manual `.dem` path entry collapsed behind a `+ 手动导入 Demo` toggle button by default, expanding the path input row on click.

#### Scenario: Toggling manual demo import
- **WHEN** the user clicks `+ 手动导入 Demo` in the `Match Library` toolbar
- **THEN** the local `.dem` path input and submit button appear below the filter bar

### Requirement: Demo parse progress bar in Match Library
The `Match Library` view SHALL display a live progress bar and stage label on any match card whose demo is currently parsing (`parse_status == "parsing"`), updated in real time from `parse.progress` IPC events.

#### Scenario: Viewing a demo currently being parsed
- **WHEN** a demo enters `parsing` state and `parse.progress` events arrive for its `demo_id`
- **THEN** the corresponding Match Library card renders a progress bar reflecting the reported percentage and current parse stage

### Requirement: Async loading progress in Match Analysis
The `Match Analysis` view SHALL display a progress bar overlay or loading card while loading match data, computing analytics, generating 10-player radar cards, or recomputing KDE heatmaps so the user has clear visual feedback during waiting periods.

#### Scenario: Opening Match Analysis for a demo
- **WHEN** `Match Analysis` begins loading a demo asynchronously
- **THEN** the view renders a progress bar with stage status (e.g., loading match stats, loading 2D replay index) until all data is ready

### Requirement: Time-accurate speed-selectable 2D tactical radar playback
The `2D Replay` tab in `Match Analysis` SHALL cache the pre-scaled map backdrop per map/layer configuration, advance frames using wall-clock elapsed time scaled by a user-selectable playback speed (`0.5x`, `1x`, `2x`, `4x`, `8x`) at the 32Hz base rate, and schedule continuous UI redraws while playing.

#### Scenario: Playing 2D tactical radar at 4x speed
- **WHEN** the user selects `4x` speed and clicks Play on `2D Replay`
- **THEN** the radar playback advances at 128 frames per second of wall-clock time (4x real-time 32Hz) smoothly without stalling behind single-frame render intervals

### Requirement: Platform filter mirrors canonical source registry
The Match Library SHALL build its platform filter options from the canonical source registry, always include an all-platforms option and an unknown-source option, and compare filter selections against stored values case-insensitively after normalization, so that legacy stored labels remain selectable.

#### Scenario: Every stored platform is filterable
- **WHEN** demos exist with `source_platform` values `faceit`, `esl`, `perfect_world`, and `5e`
- **THEN** the filter offers a matching option for each and selecting it returns exactly those demos

#### Scenario: Legacy label still matches
- **WHEN** a demo row stores the legacy value `perfectworld` and the user selects Perfect World
- **THEN** the demo is included in the result set

#### Scenario: Unknown source remains reachable
- **WHEN** the user selects the unknown-source option
- **THEN** demos stored with `unknown` are listed and demos with a recognized source are excluded

### Requirement: 2D replay combat layers

The `2D Replay` tab SHALL render shot tracers, death markers, and C4 plant/explosion overlays from the round's combat events.

#### Scenario: Shot tracers fade over time
- **WHEN** a frame is past a shot's tick by less than the fade window
- **THEN** a line from the shooter to the aim endpoint is drawn with decreasing opacity

#### Scenario: Death markers persist for the round
- **WHEN** a player dies in the selected round
- **THEN** a marker remains at the death position, with a killer-to-victim line during the recent window

#### Scenario: C4 plant and blast radius are visible
- **WHEN** the round has a plant event and playback reaches it
- **THEN** a C4 marker appears at the plant position, and after detonation a 500-unit blast circle is drawn

#### Scenario: Layers can be toggled independently
- **WHEN** the user unchecks a layer toggle
- **THEN** only that layer disappears while the remaining layers and player poses keep rendering

#### Scenario: Legacy caches render without combat layers
- **WHEN** the round comes from a v1 replay cache
- **THEN** the canvas renders poses and utility effects normally and shows no combat layers

### Requirement: Multi-map series grouping
The system SHALL derive series from parsed demos sharing the same unordered team pair and match day, orienting scores to a stable team A/B, and SHALL expose them through the CLI and the Match Library.

#### Scenario: Swapped team columns still aggregate
- **WHEN** two parsed demos share the same teams and date but one lists the teams as A/B and the other as B/A
- **THEN** the series reports two maps with scores oriented to the same team and the correct map-win totals

#### Scenario: Single-map groups are not series
- **WHEN** only one parsed demo exists for a team pair and date
- **THEN** it does not appear in the series list and stays a standalone library card

#### Scenario: CLI lists series
- **WHEN** the operator runs `strikeye-cli series --data-root <dir>`
- **THEN** the command prints a JSON document whose `series` entries include teams, date, maps, and map-win totals

#### Scenario: Library groups cards by series
- **WHEN** the "按系列赛分组" toggle is enabled and a filtered result contains maps from one series
- **THEN** a header showing `TeamA <wins>-<wins> TeamB` and the map count is rendered above that series' cards

### Requirement: Default setup heatmap dimension
The system SHALL offer a `setup` heatmap dimension that samples one replay frame per round near freeze-end + 20 seconds and includes only players alive at that frame.

#### Scenario: Twenty-second sampling
- **WHEN** a round has freeze-end tick 1000 and the demo runs at 32 tick
- **THEN** the sampled frame is the one nearest tick 1640

#### Scenario: Dead players excluded
- **WHEN** a player is dead at the sampled frame
- **THEN** that player contributes no setup point

#### Scenario: Team filter uses replay side
- **WHEN** the operator filters the setup heatmap to CT
- **THEN** only players whose replay pose team is CT are sampled

#### Scenario: Filters compose
- **WHEN** a player and a round type are selected together with `setup`
- **THEN** only that player's setup positions in matching rounds are sampled

### Requirement: Callout zone analysis tab

Match Analysis SHALL 提供「区域控制」标签页：

- 标签栏 SHALL 显示「7. 区域控制」，与既有 6 个标签并列。
- 页面 SHALL 显示采样回合数、位置样本数、已命名位置占比、记录击杀数四项 KPI。
- 页面 SHALL 显示「默认站位分布」表（区域 / T / CT / 合计 / 占比）与「击杀区位」表（回合 / 时间 / 击杀者 / 阵亡者 / HS / 区域）。
- 击杀区位最多展示 120 条，截断时 SHALL 提示可用 `strikeye-cli export` 导出完整数据。
- callout 数据缺失或地图未覆盖时 SHALL 显示下载/覆盖范围指引，而不是空白页。
- 切换 demo SHALL 重置区域数据并重新加载。

#### Scenario: Zones tab explains missing data

- **WHEN** 用户打开区域控制页但 callout CSV 不存在
- **THEN** 页面显示"尚未下载 callout 数据集"与 `callouts-fetch` 提示

#### Scenario: Zones tab renders after loading

- **WHEN** callout 数据与回放缓存齐备
- **THEN** 页面显示 4 个 KPI 与两张表，名称与计数与 zones 摘要一致

### Requirement: Freehand brush controls in the zones tab

系统 SHALL 在区域控制页提供自由笔刷控件：

- SHALL 提供开/关按钮，开启时按钮文案体现"拖动绘制"。
- SHALL 在存在当前回合笔迹时显示"清除本回合笔迹（N）"按钮。
- SHALL 显示当前回放回合号，并说明回合 0 的笔迹对所有回合可见。
- 关闭笔刷 SHALL 丢弃尚未保存的进行中笔迹。

#### Scenario: Toggle discards pending stroke

- **WHEN** 笔刷处于关闭状态且内存里有未保存点，点击开启
- **THEN** 内存中的待写点被清空

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

### Requirement: Card export and CS2 hand-off

资料库的比赛卡片 SHALL 提供导出与在 CS2 中打开两个动作：

- 「导出数据」SHALL 写出 CSV、JSON、XLSX 与 HTML 四种产物到 `<DataRoot>/media/exports`，并在状态栏显示文件数与目录。
- 导出 SHALL 复用 `internal/export` 的比赛包，不得复制列定义。
- 「用 CS2 打开」SHALL 生成 `steam://rungameid/730//+playdemo <转义后的绝对路径>` 并交给系统默认处理器。
- 平台命令 SHALL 可映射：Windows `rundll32 url.dll,FileProtocolHandler`、macOS `open`、其他 `xdg-open`。
- demo 行的文件路径为空时 SHALL 直接报错，不得调用处理器。
- 处理器返回错误时 SHALL 原样上报到状态栏。

#### Scenario: Export from a card

- **WHEN** 用户点击某场比赛卡片的「导出数据」
- **THEN** `media/exports` 下出现 CSV/JSON/HTML/XLSX，状态栏显示导出文件数与目录

#### Scenario: Open in CS2

- **WHEN** 用户点击「用 CS2 打开」且 demo 有绝对路径
- **THEN** 生成带转义路径的 `steam://` 链接并交给默认处理器

#### Scenario: Demo without a path

- **WHEN** demo 行的文件路径为空
- **THEN** 命令报错 "demo has no file path"，处理器不被调用

### Requirement: Player profile editor

分析页选手抽屉 SHALL 提供档案编辑：

- 抽屉 SHALL 显示「档案」一行，包含显示名、备注、标签（逗号分隔）三个输入与「保存档案」按钮。
- 展开选手时 SHALL 从 `player_profiles` 读取一次草稿；切换到另一位选手时 SHALL 换成该选手的草稿，不得残留上一位的文本。
- 保存 SHALL 走 `profile.Save`：空显示名/备注保留旧值，标签按中英文逗号拆分并归一化。
- 保存成功 SHALL 显示「已保存」，失败 SHALL 显示错误文本。
- 存储不可用时 SHALL 显示只读提示，不得 panic。

#### Scenario: Edit and persist

- **WHEN** 用户输入显示名 `yuurih`、备注 `support`、标签 `support, anchor` 并点击保存
- **THEN** `player_profiles` 中该 SteamID 的三个字段被写入，标签为两项

#### Scenario: Switch players

- **WHEN** 用户展开选手 A 编辑后不保存，展开选手 B
- **THEN** 输入框显示 B 的档案（或空），不显示 A 的文本

### Requirement: Map data cache controls

设置页 SHALL 提供地图数据下载入口：

- 卡片 SHALL 显示已缓存导航网格数量、callout 文件大小（缺失时显示「缺失」）与最近检查时间。
- 「下载 callout」与「下载导航网格（全部官方图）」SHALL 在后台执行，下载期间显示「下载中...」并阻止重复触发。
- 下载 SHALL 复用 `internal/mapdata` 的校验和与 `.part`→rename 逻辑。
- 成功与失败 SHALL 通过设置页状态条回报；失败时 SHALL 保留原有缓存不动。
- 缓存路径 SHALL 为 `<DataRoot>/cache/map_nav` 与 `<DataRoot>/cache/map_data/callouts.csv`。

#### Scenario: First download

- **WHEN** 缓存为空时用户点击「下载导航网格（全部官方图）」
- **THEN** 状态条显示下载中，完成后显示"导航网格已就绪 N 张"，卡片计数同步更新

#### Scenario: Failed download

- **WHEN** 下载因网络错误失败
- **THEN** 状态条显示「下载失败: …」，已存在的缓存文件不被破坏

#### Scenario: Callout missing

- **WHEN** `cache/map_data/callouts.csv` 不存在
- **THEN** 卡片显示「callout 缺失」并给出下载按钮

### Requirement: Geometry download in the map data card

设置页地图数据卡片 SHALL 覆盖碰撞几何：

- 卡片状态行 SHALL 显示已缓存碰撞几何数量（`<DataRoot>/cache/map_geometry` 下的 `.tri` 文件数）。
- 「下载碰撞几何（全部官方图）」SHALL 复用 `internal/mapgeom` 的清单校验与 `.part`→rename 下载，写入 `<DataRoot>/cache/map_geometry`。
- 下载 SHALL 在后台执行并复用卡片的进行中标志；失败时 SHALL 保留原有缓存不动并通过状态条回报。
- 任一数据缺失时卡片 SHALL 说明受影响的功能（导航网格 → 区域控制/地图控制，碰撞几何 → TTD 与击杀视线，callout → 地名）。

#### Scenario: First download

- **WHEN** 缓存为空时用户点击「下载导航网格（全部官方图）」
- **THEN** 状态条显示下载中，完成后显示"导航网格已就绪 N 张"，卡片计数同步更新

#### Scenario: Geometry download

- **WHEN** 用户点击「下载碰撞几何（全部官方图）」
- **THEN** `cache/map_geometry` 出现 `.tri` 文件，状态行几何计数增加，TTD 分析可算出结果

#### Scenario: Failed download

- **WHEN** 下载因网络错误失败
- **THEN** 状态条显示「下载失败: …」，已存在的缓存文件不被破坏

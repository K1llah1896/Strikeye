# Spec Delta

## Purpose

定义地图导航数据的获取、解析与区域语义，使坐标可以被解释为地名与可行走空间。

## ADDED Requirements

### Requirement: Pinned navigation and callout datasets

系统 SHALL 从钉定版本获取导航与 callout 数据，并在使用前校验完整性：

- 导航数据来自 `awpy-data` release `2000950` 的 `navs.zip`，按 manifest 的 `bytes` 与 `sha256` 校验。
- callout 数据来自 `cs2-server-admin-callouts` 的 `schematic/data/nav_places.csv`（commit 钉定），按 `size` 与 `sha256` 校验。
- 下载写入 `<DataRoot>/cache/` 下的临时文件，校验通过后原子重命名；已缓存且校验匹配时不联网。
- 二进制数据不进入仓库。

#### Scenario: First fetch downloads and verifies

- **WHEN** 用户执行 `strikeye-cli navs-fetch --map de_dust2`
- **THEN** 系统下载 navs.zip、校验大小与 sha256、解压出 `cache/map_nav/de_dust2.nav`，并在输出中列出已缓存地图

#### Scenario: Corrupted download leaves no artifact

- **WHEN** 下载内容与 manifest 的 sha256/max size 不符
- **THEN** 命令返回校验错误，且 `cache/map_nav` 下不出现半成品 `.nav` 文件

### Requirement: Nav mesh parsing and point lookup

系统 SHALL 解析 CS2 `.nav` v30–v36 并提供点定位与寻路：

- 版本不在支持区间或结构校验失败时返回错误，不 panic。
- 点定位按 XY 盒命中并结合 centroid Z 选择楼层，避免上下层同 XY 串位。
- 寻路使用 centroid 距离的 Dijkstra，返回区域序列与总距离。

#### Scenario: Locate a player on the lower floor

- **WHEN** 同一 XY 存在上下两层区域，查询点 Z 更接近下层
- **THEN** 返回下层的 area id

#### Scenario: Path between callouts

- **WHEN** 用户执行 `strikeye-cli navpath --map de_dust2 --from TSpawn --to CTSpawn`
- **THEN** 输出跳数、单位距离、米数与按 250 u/s 计算的跑图秒数

### Requirement: Callout zone attribution

系统 SHALL 把世界坐标归属到 callout 地名：

- 命中多个盒时取面积最小者；Z 偏差超过 48 单位不命中。
- 无盒命中时回退到 512 单位内最近盒心，并标记为"邻近"（低置信）而非精确命中；仍无命中则不归属（UI 显示 `—`）。
- 地图名匹配忽略大小写与 `.dem`/`.nav` 后缀。

#### Scenario: Snap fallback is reported as inexact

- **WHEN** 一个存活选手的坐标不在任何盒内但距最近盒心 100 单位
- **THEN** 归属该盒的 callout，并计入"邻近"计数而非"精确"计数

#### Scenario: Overlapping boxes resolve to the specific area

- **WHEN** 一个点同时落在 `BombsiteA` 与 `LongA` 两个盒内
- **THEN** 归属为面积更小的 `LongA`

### Requirement: Zone summaries from replay frames

系统 SHALL 基于回放帧产出区域摘要：

- 默认站位：每回合在 `freeze_end + 20s` 就近取帧，统计存活选手的 callout，按 T/CT 分列。
- 击杀区位：每条死亡事件按阵亡者位置归属 callout，含回合、时间、击杀者、阵亡者、是否爆头。
- 缺少回放帧或 callout 时返回空摘要而非报错；解析流程不受影响。

#### Scenario: Setup distribution counts only alive players

- **WHEN** 采样帧中某选手已阵亡
- **THEN** 该选手不计入任何 callout 计数

### Requirement: CLI navigation commands

系统 SHALL 提供 `navs`、`navs-fetch`、`callouts-fetch`、`area`、`navpath` 五个子命令，输出 JSON，并在数据缺失时给出可执行的下一步提示。

#### Scenario: Area query without dataset

- **WHEN** 用户执行 `area` 但 callout 数据尚未下载
- **THEN** 命令提示先运行 `callouts-fetch`，退出码非零

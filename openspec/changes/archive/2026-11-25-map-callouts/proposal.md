# Change: Map Navigation Data and Callout Zones

## Why

Strikeye 能画出 2D 回放和热区，但所有位置都是匿名坐标：用户看到"这里死了三次"，却不知道那是 A 大还是中路。调研报告把 navmesh 与 callout 列为外部阻塞项，tick 数据本身不含地名——

- CS2 的 `.nav` 只有导航网格（v30–v36，见 navmesh 解析），地名从 CS:GO 时代就已移出 nav；
- callout 名称位于地图实体的 `env_cs_place`，必须单独取得一份"世界坐标 → 地名"数据表。

两份数据都可以从公开仓库按版本钉定取得（`pnxenopoulos/awpy-data` 的 navs.zip、`sink3x/cs2-server-admin-callouts` 的 nav_places.csv），因此不再需要用户手工提供文件。

## What Changes

- 新增 `internal/navmesh`：解析 CS2 `.nav`（v30–v36，跳过 v36 内嵌占位 BinaryKV3），提供点定位（含上下楼层区分）与 Dijkstra 寻路。
- 新增 `internal/mapdata`：钉定 awpy-data release（`2000950`）与 callout 数据集 commit，下载 navs.zip 并校验 `bytes`+`sha256`，按需解压单图 `.nav`；callout CSV 同样按大小与来源校验缓存。
- 新增 `internal/callouts`：加载 `map,name,x1,y1,z1,x2,y2,z2` 表，提供盒命中（优先最小面积、Z 容差 48）与 512 单位内最近盒心回退。
- 新增 `internal/zones`：把回放帧转成区域摘要——开局 20 秒的默认站位分布、按倒地位置归属的击杀区位。
- CLI 新增 `navs`、`navs-fetch`、`callouts-fetch`、`area`、`navpath`。
- Match Analysis 新增「7. 区域控制」标签页：KPI + 默认站位分布表 + 击杀区位表；数据缺失时给出下载指引而不是空白。

## Impact

- Affected specs: `map-navigation`（新增）、`match-library-and-analysis-ui`（修改）
- Affected code: `internal/navmesh`、`internal/mapdata`、`internal/callouts`、`internal/zones`、`cmd/strikeye-cli`、`internal/ui/views/analysis`
- 数据许可：nav 几何来自 awpy-data（Valve 地图资产衍生），callout 表为 MIT；二进制不入库，仅在用户执行 fetch 时写入 `<DataRoot>/cache/`。

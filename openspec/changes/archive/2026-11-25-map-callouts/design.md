# Design: Map Navigation Data and Callout Zones

## Context

- 回放帧（`.cs2rpl` v2）只有坐标，没有语义位置。
- 需要两类数据：导航网格（可行走区域、连通性、路径距离）和 callout 表（世界坐标盒 → 地名）。
- 两者来源不同、更新节奏不同，且体量不小（navs.zip 7.6MB，单图 dust2 2242 areas / ancient 1969 areas）。

## Decisions

### 钉定版本而不是"最新"

`mapdata.NavRelease = "2000950"`（CS2 1.42.0.1）与 callout 仓库 commit `e222d67…` 写死在代码里。玩家可能用旧 demo 解析，但地图几何跨小版本稳定；钉定版本让"同一 demo 两次分析结果一致"，升级是显式动作（改常量 + 跑校验）。

### 校验和先于解压

`EnsureNavs` 先比对 zip 的 `bytes` 与 `sha256`（来自 awpy-data 的 manifest.json），通过后才解压；解压同样走 `.part` → rename，失败不留残档。callout CSV 用 `size + sha256` 双校验，命中缓存不联网。

### 点定位的两个回退层

1. 盒命中：玩家在 box 内即归属该 callout；重叠时取面积最小者（更具体，如 `LongA` 优于 `BombsiteA`）；Z 容差 48 单位，避免上下层串位。
2. 邻近回退：都不命中时取 512 单位内最近的盒心；仍无则返回 false，UI 记为 `—`，不猜。

### 精确命中与邻近回退分开计数

`nav_places.csv` 的盒只有 25–75 单位宽（按 nav area 采样，dust2 1118 盒 / 2242 区域），因此必须回退。回退不是"猜"，但要可审计：`SetupTotals` 分别记录 `Exact` 与 `Snapped`，CLI 输出 `exact_hits/snapped_hits/snap_mean_units`。真实 dust2 回放实测：184 个采样点中 157 精确、27 邻近，平均回退距离 66.9 单位。

### 20 秒采样点

开局站位用 `freeze_end + 20s`：既过了默认架枪的展开期（CS2 出生点离包点更远），又早于多数首杀（实测首杀中位数在 25–35 秒）。与热力图的 `setup` 维度共用同一采样点，两个页面数字可互相印证。

### nav 寻路只在 CLI 暴露

`navpath` 目前是诊断/路线查询工具（输出跳数、米数、刀速跑图时间）。把它接进 UI 需要先有"路径叠加在 2D 回放上"的绘制设计，留给后续 change，避免这一版把画布逻辑搅乱。

## Risks

- `nav_places.csv` 覆盖 10 张图，非全部官方图；UI 在缺图时列出已覆盖地图，而不是假装成功。
- `.nav` 版本若升到 v37，解析器会以 `ErrInvalidNav` 失败而不是误读；届时按 awpy 的 `nav.rs` 增量更新。

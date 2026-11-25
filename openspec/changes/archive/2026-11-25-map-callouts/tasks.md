# Tasks

## 1. Navigation data
- [x] 1.1 `internal/navmesh`：v30–v36 解析、点定位、Dijkstra 寻路
- [x] 1.2 v36 内嵌占位 BinaryKV3 跳过与结构校验
- [x] 1.3 `internal/mapdata`：awpy-data release 钉定、manifest 校验、按需解压
- [x] 1.4 下载校验失败不留残档

## 2. Callout data
- [x] 2.1 `internal/callouts`：CSV 加载、盒命中（最小面积 + Z 容差）、邻近回退
- [x] 2.2 callout 数据集下载与 `size + sha256` 校验
- [x] 2.3 地图名大小写与后缀容错

## 3. Zone summaries
- [x] 3.1 `internal/zones`：开局 20 秒站位分布（T/CT 分列）
- [x] 3.2 击杀区位：按阵亡者位置归属 callout
- [x] 3.3 缺少回放/callout 时返回空摘要

## 4. CLI
- [x] 4.1 `navs` / `navs-fetch` / `callouts-fetch`
- [x] 4.2 `area` / `navpath`（含米数与跑图秒数）
- [x] 4.3 缺数据时提示下一步

## 5. UI
- [x] 5.1 区域控制标签页与 4 个 KPI
- [x] 5.2 默认站位分布表与击杀区位表
- [x] 5.3 缺数据空状态与 demo 切换重置

## 6. Verification
- [x] 6.1 单元测试：navmesh / mapdata / callouts / zones
- [x] 6.2 真实数据：dust2 2242 areas、ancient 1969 areas；TSpawn→CTSpawn 5168.58u / 131.28m / 20.67s
- [x] 6.3 `go test ./...` 与 `go vet` 通过
- [x] 6.4 更新调研报告的数据来源与阻塞项

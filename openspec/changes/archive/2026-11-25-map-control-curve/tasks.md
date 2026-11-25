# Tasks

## 1. Control model
- [x] 1.1 `internal/mapcontrol`：按队伍多源 Dijkstra（截断半径）
- [x] 1.2 争夺/归属/中立判定与净值
- [x] 1.3 从 freeze time 起采样，默认 5 秒

## 2. Callout aggregation
- [x] 2.1 区域质心归属 callout
- [x] 2.2 每 callout 输出平均/最小/最大净值
- [x] 2.3 缺 callout 数据时仍输出整图曲线

## 3. CLI
- [x] 3.1 `mapcontrol --demo-id --step --radius`
- [x] 3.2 缺 nav 时提示 `navs-fetch`

## 4. UI
- [x] 4.1 区域控制页控制卡片（sparkline + 平均净值 + CT 占优占比）
- [x] 4.2 按偏离中立程度排序的区域条形图
- [x] 4.3 缺 nav 时空状态提示

## 5. Verification
- [x] 5.1 单元测试：控制归属（分队/争夺/中立）、聚合、sparkline/条形图、分桶
- [x] 5.2 真实数据：dust2 19 回合 409 采样，0.6 秒完成；CT 净值 +0.2 左右，B Site/Long A 偏 CT、Upper Tunnel 偏 T
- [x] 5.3 触发式重解析后回放缓存含弹道/死亡/C4（此前缓存为旧版本，已确认 137 条击杀）
- [x] 5.4 `go test ./...` 通过

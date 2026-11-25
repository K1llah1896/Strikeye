# Tasks

## 1. Sampling

- [x] 1.1 `visSample` 增 `Yaw`/`Blind`，新增 `sampleObservation`
- [x] 1.2 `single_pass.go` 采样改用它

## 2. Visibility

- [x] 2.1 `visibleRunMSOptics` + `ttdFOVHalfAngle=53` + `ttdBlindThreshold=0.1`
- [x] 2.2 遮挡/致盲/背身任一不满足即中断游程
- [x] 2.3 `angleBetweenDeg` 工具函数

## 3. Verification

- [x] 3.1 单测：正对可见、背身不可见、20°/53° FOV 边界、致盲与残余闪光、角度工具
- [x] 3.2 真实 dust2 重解析（NiKo TTD 1263.7→940.9ms/23）
- [x] 3.3 `go test ./...`、`openspec validate ttd-optics --strict`

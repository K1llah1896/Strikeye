# Tasks

- [x] 回放格式 v3：`ReplayPlayerPose.Zoom` 读写（v2 兼容）
- [x] 解析采样写入 `ActiveWeapon().ZoomLevel()`
- [x] 可见性：`visSample.Zoom` + `scopedFOVHalfAngle`
- [x] 击杀光学：`AttackerZoom/VictimZoom` + 各自视锥判定
- [x] 测试：v3 往返、v2 读回、半角表、可见性收窄、击杀光学收窄
- [x] 真实数据：重解析 dust2（v3 缓存），zoom 分布 0/1/2 均出现
- [x] `corpus --update` 重生成黄金快照并复跑 0 差异
- [x] `docs/metrics.md` 与调研报告 §4.4/§9 同步
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`

# Tasks

## 1. Effects model
- [x] 1.1 `Config.ApplyEffects` / `Config.SmokeRadius`，默认开启与 144 单位
- [x] 1.2 `activeEffects` 计算烟雾遮蔽集合与火焰封路集合
- [x] 1.3 火焰节点从 Dijkstra 展开中摘除
- [x] 1.4 `Frame.blocked` 输出

## 2. CLI
- [x] 2.1 `mapcontrol --effects`（默认 true）
- [x] 2.2 复用 `loadReplayRounds`，去掉重复的回放解码

## 3. UI
- [x] 3.1 控制卡片显示遮挡区域平均数
- [x] 3.2 说明文案点明遮挡区域不计入归属

## 4. Verification
- [x] 4.1 单测：烟雾遮挡不计归属、火焰阻断穿行、关闭效果回到旧模型、凸包射线法
- [x] 4.2 真实数据：dust2 409 采样平均遮挡 16.9 区域（峰值 111），净值 +0.20 → +0.156
- [x] 4.3 `go test ./...`、`go vet ./...` 通过

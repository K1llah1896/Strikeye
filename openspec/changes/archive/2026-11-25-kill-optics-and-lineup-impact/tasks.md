# Tasks

## 1. Kill optics
- [x] 1.1 `Config.FOVHalfAngle` 与 `BlindThreshold`
- [x] 1.2 `poseAt` 取击杀前最近帧、`angleDeg` 视线夹角
- [x] 1.3 `aim_off_deg` / `in_fov` / `victim_sees_killer`
- [x] 1.4 `attacker_blind` / `victim_blind`（剩余致盲 > 0.1s）
- [x] 1.5 汇总 `optics_used` / `fov_pct` / `ambush_pct` / `blind_kills`
- [x] 1.6 单测：FOV、背身、闪光、无帧降级

## 2. Line-up impact
- [x] 2.1 聚类记录获胜回合与队伍
- [x] 2.2 同队同侧基准胜率
- [x] 2.3 `wins` / `win_rate` / `impact` / `kills_per_throw`
- [x] 2.4 排序规则（次数优先，其次收益）
- [x] 2.5 单测：66.7% 胜率、+16.7 收益、平局不计入分子

## 3. UI
- [x] 3.1 击杀区位表追加 `·背身`、`·盲`
- [x] 3.2 道具线位表新增「回合胜率」「收益」列与着色

## 4. Verification
- [x] 4.1 真实数据：dust2 背身率 12.4%、致盲击杀 15、`fov_pct` 100%（校验 yaw 可信）
- [x] 4.2 真实数据：长 A 闪光 +20.5、中门烟 −3.8 等收益排序
- [x] 4.3 `go test ./...`、`go vet ./...`、`openspec validate --strict` 通过

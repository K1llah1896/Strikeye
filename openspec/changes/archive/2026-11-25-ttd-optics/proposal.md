## Why

TTD 目前只做"眼位到眼位的 BVH 射线"，等于假设选手 360° 都能看见。真实对枪里，背身、开镜方向、刚被闪白都会让"看得见"不成立；报告 4.4 与 §9 因此把 TTD 的光学修正列为待办。

击杀级已经算过 yaw/FOV/致盲（`kill-optics-and-lineup-impact`），本轮把同一套判据前移到每帧可见性采样，让 TTD 与 TTD 的含义一致。

## What Changes

- `visSample` 增加 `Yaw`（眼角 yaw，度，0=+X 逆时针正）与 `Blind`（剩余致盲秒数），采样函数改为 `sampleObservation`。
- `visibleRunMSOptics(tracer, samples, att, vic, tick, window, tickRate, fovHalfDeg)`：一帧只有同时满足"无遮挡 + 攻击者未致盲 + 受害者在攻击者水平 FOV 内"才算可见；任一条件破坏即中断游程。
- 常量 `ttdFOVHalfAngle = 53`、`ttdBlindThreshold = 0.1`，与 `killctx` 的取值一致。
- 距离小于 8 单位时跳过 FOV 判定（方向不可定义）。
- 旧签名 `visibleRunMS` 保留，等价于 53° FOV。

## Impact

- 修改：`internal/parser/visibility.go`、`internal/parser/single_pass.go`（采样点）。
- 不改变 `.cs2rpl` 格式、不改迁移：TTD 仍是 `ttd_avg_ms/ttd_samples` 两列。
- TTD 数值会下降（更严格），历史数据需重解析才一致。

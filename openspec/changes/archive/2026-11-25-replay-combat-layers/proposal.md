# Proposal: 回放交火层（弹道/击杀/C4）

## Why

当前 2D 回放只画烟雾、燃烧弹、投掷物轨迹与选手位姿，缺少同类工具（cs-demo-manager、cs-demo-analyzer）默认提供的交火信息：射击弹道、死亡位置、C4 安放/拆除/爆炸范围。观众无法从回放中读出"谁在何时从何处开火、谁倒在哪里、包点爆炸覆盖哪里"。

## What Changes

- CS2RPL01 格式升级到 v2，追加交火块（射击、死亡、C4 事件），读取端兼容 v1。
- 单遍解析新增 `WeaponFire`、击杀位置、C4 安放/拆除/爆炸位置采样。
- 2D 画布新增弹道、死亡标记、C4 安放点与爆炸范围三个图层，并提供图层开关与图例。
- 保持既有烟雾/燃烧/轨迹图层的绘制顺序与性能预算。

## Impact

- Affected specs: `replay-and-input-stream`（MODIFIED）、`match-library-and-analysis-ui`（ADDED）
- Affected code: `internal/parser/replay_binary.go`、`internal/parser/single_pass.go`、`internal/ui/components/radar_2d_canvas.go`、`internal/ui/views/analysis/analysis.go`

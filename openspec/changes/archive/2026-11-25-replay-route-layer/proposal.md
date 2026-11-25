# 回放推进路线图层（补记 spec）

## 为什么

2D 画布早就有「推进路线」图层（`Radar2DCanvas.layers.Route`），实现里每 8 tick（64 tick 下 4Hz）采样一次选手位置、按队伍着色连成折线、末点画当前位置，并在 2D 回放页有对应复选框。但 spec 库里没有这条 requirement——代码领先于设计文档，后续改这块时没有可校验的口径。本 change 把它补记进 `replay-and-input-stream`。

## 变什么

1. 在 `replay-and-input-stream` ADDED「Route trails」：采样步长、着色、末点、开关默认值与关闭后的行为。
2. 明确它与 `navpath` 的分工：路线图层是"实际走过哪里"，`navpath` 是"navmesh 认为怎么走"。

## 影响

- capability：`replay-and-input-stream`（ADDED「Route trails」）。
- 仅文档与既有行为对齐，无代码改动；测试 `internal/ui/components/radar_2d_layers_test.go` 的 `TestRouteLayerDrawsTeamTrails` 已覆盖。

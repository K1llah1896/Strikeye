# Spec Delta

## ADDED Requirements

### Requirement: Lower-level alignment controls

2D 画布 SHALL 支持把下层雷达按偏移与透明度合成为底图：

- `SetLowerView(offsetX, offsetY, opacity)` SHALL 钳制偏移到 ±512、透明度到 (0,1]，并在参数变化时重建背景缓存。
- 默认参数（0,0,1）下的渲染 SHALL 与未设置时逐像素一致。
- 分析页 SHALL 在回放与热力图两个面板提供下层 X/Y 偏移与不透明度控件，以及「重置」与「下层半透明」快捷按钮。
- 改动 SHALL 立即持久化，并在选中 demo 时按地图名载入。

#### Scenario: Offset shifts the underlay
- **WHEN** the lower view is moved by 24/-16 px
- **THEN** the rendered backdrop changes

#### Scenario: Reset restores the original backdrop
- **WHEN** offsets are returned to 0,0 with opacity 1
- **THEN** the rendered backdrop matches the pre-adjustment frame byte for byte

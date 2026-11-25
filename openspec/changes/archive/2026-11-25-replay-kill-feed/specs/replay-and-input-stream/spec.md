# Spec Delta

## ADDED Requirements

### Requirement: Kill feed overlay

2D 回放画布 SHALL 能把最近的击杀渲染成屏内信息条：

- 只 SHALL 显示当前 tick 之前、且在 6 秒窗口内的击杀，最多 4 条，按时间升序排列（最新在底部）。
- 每条 SHALL 显示击杀者与死者的选手名，爆头时 SHALL 追加红色标记。
- 武器名 SHALL 由击杀者最接近的弹道事件推断（±64 tick 内），推断不到时 SHALL 省略。
- 缺少 slot→姓名映射时 SHALL 回退为 `#<slot>`。
- 超过窗口的击杀 SHALL 完全不再绘制（与关闭该图层的结果逐像素一致）。

#### Scenario: Recent kills are listed
- **WHEN** a round has a kill 4 ticks before the rendered frame
- **THEN** the canvas draws a feed row naming killer and victim

#### Scenario: Expired kills disappear
- **WHEN** the rendered frame is 6+ seconds after the last kill
- **THEN** the frame is identical to one rendered with the kill feed disabled

# Tasks

- [x] 1.1 迁移 v9 `map_strokes` + 索引；`MapStrokesRepo`（Add/List/Delete/DeleteByRound、坐标裁剪、坏行容错）
- [x] 1.2 `Radar2DCanvas`: `Stroke`/`StrokePoint`、`SetStrokes`、`SetBrush`、`drawStrokesLocked`、`clampNorm`
- [x] 1.3 `RenderUI` 指针采样（`PointerPosition`/`Pressed`/`PressedOutside`）
- [x] 1.4 `analysis_strokes.go`: `applyStrokes`、`strokesForRound`、`handleBrushPoint`（0.004 步长、释放落库）、`toggleBrush`、`clearRoundStrokes`、`renderStrokeControls`
- [x] 1.5 区域控制页"自由笔刷"卡片；切换回放回合时同步笔迹
- [x] 1.6 `strikeye-cli strokes --demo-id [--round] [--clear]`
- [x] 2.1 测试：storage repo、canvas 像素差与单点忽略、brush 状态、`strokesForRound`、释放落库、toggle 丢弃
- [x] 2.2 `go test ./...` 全绿；`openspec validate --all --strict` 通过

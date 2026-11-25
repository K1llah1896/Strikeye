# Proposal: 2D 雷达自由笔刷（画板）

## Why

调研报告把"2D 画板"记为"技术不可行：mygo 拿不到画布内鼠标坐标"。这个结论是错的：mygo v0.10 的 `Element` 提供 `PointerPosition() (x, y float32, over bool)`、`Pressed()` 与 `Dragged()`，元素只要读取指针位置就会收到指针移动帧。也就是说，自由绘制在当前 UI 栈完全可以做。

战术画板是 2D 回放里仍然缺的一块：教练想在雷达图上画个箭头表示"队伍从这里压"，或者在默认站位上圈出对手的架点区域，而不只是往 callout 钉一个固定标记。

## What Changes

- 画布组件新增 `Stroke`/`StrokePoint`（归一化 [0,1] 坐标，抗窗口缩放）、`SetStrokes`、`SetBrush(active, handler)`，并在帧绘制后叠加笔迹（3px、圆头、四色可选）。
- RenderUI 在笔刷开启时每帧读取指针位置与按下状态，通过回调交给视图层。
- 存储：迁移 v9 `map_strokes(demo_id, round_number, color, points_json, created_at)`，`MapStrokesRepo` 提供 Add/List/Delete/DeleteByRound。
- 分析页「区域控制」新增"自由笔刷"卡片：开关按钮、当前回合笔迹计数、清除本回合；笔迹按 demo + 回合持久化，切换回放回合自动切换笔迹（回合 0 对全部回合可见）。
- CLI `strokes --demo-id [--round] [--clear]`：无 UI 环境下检查/清理笔迹。

## Impact

- Affected specs: `replay-and-input-stream`（新增画板 requirement）、`match-library-and-analysis-ui`（区域控制页新增卡片）
- Affected code: `internal/ui/components/radar_2d_canvas.go`、`internal/storage/{migrations.go,map_stroke_repo.go}`、`internal/ui/views/analysis/analysis_strokes.go`、`cmd/strikeye-cli/strokes.go`

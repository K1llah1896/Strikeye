## Why

报告 §9 把"2D 自由画板"列为未补齐：cs-demo-manager 可以在图上随手圈点讲战术。Strikeye 的 UI 栈（mygo v0.10.0）只给元素级 `OnClick`，拿不到画布内的鼠标坐标，自由笔刷做不了；但"把某个区域标出来并写一句话"是可以做到的——把标记钉在 callout 上，位置由数据集给出，持久化到本地库。

## What Changes

- 迁移 v7 新增 `map_annotations`（demo_id、round_number、callout_id、note、color、created_at），唯一键 `(demo_id, round_number, callout_id)`。
- `storage.MapAnnotationsRepo`：`Add`（按键 upsert）、`Get`、`ListByDemo`、`DeleteByKey`、`Delete`；颜色归一化为 amber/red/green/blue。
- 雷达画布新增 `Pin{X,Y,Label,Color}` 与 `SetPins`，在回放帧之上绘制圆圈+地名；无回放帧时仍绘制（区域标记与具体回合无关）。
- 区域控制页：默认站位表每行前加 ◎/◉ 按钮切换标记（写库并即时重绘雷达）；新增「战术标记」卡片列出标记与备注，可逐条移除。
- CLI `notes --demo-id --round --callout --note --color --delete --clear`，始终回显当前标记列表。

## Impact

- 新增：`internal/storage/map_annotation_repo.go`、`cmd/strikeye-cli/notes.go`。
- 修改：迁移、`radar_2d_canvas.go`、`analysis_zones.go`、`zones.Row`（新增 `PlaceID`，`SetupTotals.IDs` 记录标签→id）。
- 不改变回放缓存与既有指标。

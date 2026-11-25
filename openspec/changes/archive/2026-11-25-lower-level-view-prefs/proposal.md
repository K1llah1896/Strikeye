# Proposal: 下层雷达的对齐与透明度持久化

## Why

报告 §4.2 指出 CS:DM 的上下层雷达能"每地图保存偏移与透明度"。我们只有上层/下层二选一切换：一旦切到下层，当前楼层就消失，而两层之间常见的错位（不同地图的 radar 原点不一致）无法校正。

## What Changes

- 迁移 v11 新增 `map_view_prefs(map_name PRIMARY KEY, lower_offset_x, lower_offset_y, lower_opacity, updated_at)`。
- `storage.MapViewPrefsRepo`：`Get`（缺行返回居中/不透明默认）、`Upsert`（偏移钳制 ±512、透明度钳制 0.1–1.0）、`List`。
- 画布 `SetLowerView(offsetX, offsetY, opacity)`：下层按偏移平移并以统一 alpha 合成，默认值下逐像素等于原背景。
- 分析页回放与热力图两处都提供「下层 X / Y / 不透明度」滑杆与「重置」「下层半透明」按钮，改动即持久化；选中 demo 时按地图载入。
- CLI `viewprefs [--set --map --x --y --opacity]`。

## Impact

- Affected specs: `sqlite-storage-and-migration`、`replay-and-input-stream`
- Affected code: `internal/storage/{migrations.go,map_view_prefs_repo.go}`、`internal/ui/components/radar_2d_canvas.go`、`internal/ui/views/analysis/{analysis.go,analysis_lower_view.go}`、`cmd/strikeye-cli/{main.go,viewprefs.go}`

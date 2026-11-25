# Tasks

## 1. Storage

- [x] 1.1 迁移 v7 + `map_annotations` 表与索引
- [x] 1.2 `MapAnnotationsRepo`（upsert/get/list/delete/deleteByKey、颜色归一化）

## 2. Rendering

- [x] 2.1 画布 `Pin` / `SetPins` / `drawPinsLocked`（四色、圆圈、地名）
- [x] 2.2 无回放帧时仍绘制

## 3. UI

- [x] 3.1 `zones.SetupTotals.IDs` 与 `Row.PlaceID`
- [x] 3.2 站位表 ◎/◉ 切换 + 战术标记卡片 + 移除
- [x] 3.3 `pinsFromAnnotations` / `applyPins` / `toggleAnnotation` / `removeAnnotation`

## 4. CLI

- [x] 4.1 `notes`（增/删/清空/列表，空列表序列化为 `[]`）

## 5. Verification

- [x] 5.1 storage 测试（upsert、颜色回退、按 demo 过滤、双删除）
- [x] 5.2 canvas 测试（有/无 pins 像素差、清空复原）
- [x] 5.3 analysis 测试（pin 坐标与标签、缺失 callout 跳过、颜色区分）
- [x] 5.4 CLI 测试（增改删清 + 缺失 id 报错）
- [x] 5.5 `go test ./...`、`openspec validate map-annotations --strict`

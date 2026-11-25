# Spec Delta

## ADDED Requirements

### Requirement: Per-map view preference storage

数据库 SHALL 持久化每张地图的 2D 下层对齐参数：

- 迁移 SHALL 新增 `map_view_prefs` 表，主键为地图名，字段包含 `lower_offset_x`、`lower_offset_y`、`lower_opacity`、`updated_at`。
- 读取缺失行时 SHALL 返回偏移 0、透明度 1 的默认值而不报错。
- 写入 SHALL 把偏移钳制在 ±512、透明度钳制在 0.1–1.0，并要求地图名非空。
- 列表 SHALL 按地图名排序。

#### Scenario: Defaults for an unseen map
- **WHEN** preferences are read for a map that was never configured
- **THEN** offsets are zero and opacity is 1

#### Scenario: Out-of-range values are clamped
- **WHEN** a caller stores offset 9999 and opacity 7
- **THEN** the stored row reads back as offset 512 and opacity 1

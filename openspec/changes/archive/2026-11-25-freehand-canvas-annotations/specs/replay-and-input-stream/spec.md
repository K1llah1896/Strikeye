# Spec Delta

## Purpose

在 2D 回放雷达图上提供自由笔刷画板：拖动即画、松手保存、按 demo 与回合持久化。

## ADDED Requirements

### Requirement: Freehand stroke storage

系统 SHALL 持久化自由笔迹：

- 迁移 SHALL 建立 `map_strokes(demo_id, round_number, color, points_json, created_at)` 并索引 `(demo_id, round_number)`。
- 笔迹点 SHALL 存为归一化 `[0,1]` 坐标，写入时 SHALL 裁剪到该区间。
- 少于两个点的笔迹 SHALL 被拒绝；缺少 demo id SHALL 被拒绝。
- 颜色 SHALL 限定 amber/red/green/blue，非法值归为 amber。
- 读取时坏 JSON 行 SHALL 返回空点集而不是让整个查询失败。
- SHALL 支持按 id 删除与按 (demo, round) 批量删除。

#### Scenario: Coordinates are clamped

- **WHEN** 写入点 `(-1, 2)`
- **THEN** 存储为 `(0, 1)`

#### Scenario: Single point rejected

- **WHEN** 写入只含一个点的笔迹
- **THEN** 返回错误且不落库

#### Scenario: Corrupt row tolerated

- **WHEN** `points_json` 不是合法 JSON
- **THEN** `List` 仍返回该行，`points` 为空

### Requirement: Freehand drawing on the 2D canvas

系统 SHALL 在 2D 回放画布上支持自由绘制：

- 画布 SHALL 接受归一化笔迹并按画布尺寸缩放绘制，线宽 3、圆头，绘制在选手与标记之后。
- 少于两点的笔迹 SHALL NOT 绘制。
- 笔刷开启时，画布 SHALL 在指针位于画布内且主键按下时每帧回调归一化坐标与按下状态；指针拖出画布仍保持按下时 SHALL 继续回调。
- 拖动期间采样点 SHALL 与上一点距离小于 0.004 时跳过。
- 松开主键 SHALL 把整条笔迹一次性写入存储并刷新画布。

#### Scenario: Stroke changes pixels

- **WHEN** 画布设置一条从 (0.1,0.1) 到 (0.9,0.9) 的三点笔迹并重绘
- **THEN** 输出图像与无笔迹时存在像素差异

#### Scenario: Sampling throttles near-duplicate points

- **WHEN** 连续回调 (0.1,0.1) 与 (0.1005,0.1005)
- **THEN** 只记录第一个点

#### Scenario: Release persists

- **WHEN** 拖动后松开主键
- **THEN** 数据库新增一条笔迹，内存中的待写点清空，画布显示该笔迹

### Requirement: Stroke CLI

系统 SHALL 提供 `strikeye-cli strokes --demo-id <id>`：

- SHALL 输出该 demo 的笔迹列表与数量。
- `--round n` SHALL 只返回该回合的笔迹。
- `--clear` SHALL 删除匹配的笔迹并输出删除数量。
- 缺少 `--demo-id` SHALL 返回错误。

#### Scenario: Clear one round

- **WHEN** 以 `--demo-id 2 --round 3 --clear` 运行
- **THEN** 返回 `{"removed": n, "round": 3}`

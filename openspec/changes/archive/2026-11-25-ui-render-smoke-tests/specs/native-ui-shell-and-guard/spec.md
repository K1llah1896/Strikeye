# native-ui-shell-and-guard

## ADDED Requirements

### Requirement: Headless render smoke tests

视图 SHALL 具备无头渲染回归测试：

- `internal/ui/renderutil` SHALL 提供 `Probe` 与 `AssertRendered`，用 `ui.Render` 在 1280×800 渲染并按 4px 采样统计不同颜色数。
- 资料库、设置、分析、精彩回放、观战五个视图 SHALL 各至少有一个渲染测试，渲染结果颜色数低于阈值时 SHALL 失败。
- 分析视图 SHALL 逐一渲染全部标签页（总览/经济/表现/雷达/2D 回放/热力图/区域控制/队伍视角）。
- 测试数据 SHALL 由临时 SQLite 播种，不依赖真实 demo 文件。

#### Scenario: Blank frame detected

- **WHEN** 某视图因 nil 指针或空数据只画出背景
- **THEN** 渲染测试因颜色数不足失败，而不是通过

#### Scenario: Panel branches covered

- **WHEN** 下载面板、API Key 面板或 SQL 结果表格处于展开状态
- **THEN** 渲染测试覆盖这些分支并保持通过

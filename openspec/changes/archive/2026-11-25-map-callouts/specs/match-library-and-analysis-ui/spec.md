# Spec Delta

## Purpose

在比赛分析中把坐标翻译成地名，使"哪里死了人、谁常站哪里"可以直接读出。

## ADDED Requirements

### Requirement: Callout zone analysis tab

Match Analysis SHALL 提供「区域控制」标签页：

- 标签栏 SHALL 显示「7. 区域控制」，与既有 6 个标签并列。
- 页面 SHALL 显示采样回合数、位置样本数、已命名位置占比、记录击杀数四项 KPI。
- 页面 SHALL 显示「默认站位分布」表（区域 / T / CT / 合计 / 占比）与「击杀区位」表（回合 / 时间 / 击杀者 / 阵亡者 / HS / 区域）。
- 击杀区位最多展示 120 条，截断时 SHALL 提示可用 `strikeye-cli export` 导出完整数据。
- callout 数据缺失或地图未覆盖时 SHALL 显示下载/覆盖范围指引，而不是空白页。
- 切换 demo SHALL 重置区域数据并重新加载。

#### Scenario: Zones tab explains missing data

- **WHEN** 用户打开区域控制页但 callout CSV 不存在
- **THEN** 页面显示"尚未下载 callout 数据集"与 `callouts-fetch` 提示

#### Scenario: Zones tab renders after loading

- **WHEN** callout 数据与回放缓存齐备
- **THEN** 页面显示 4 个 KPI 与两张表，名称与计数与 zones 摘要一致

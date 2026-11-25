# Spec Delta

## Purpose

在分析页选手抽屉里呈现选手的跨场走势（Rating/ADR sparkline、首尾差值、最近对手），让球探不必离开比赛视图就能看到状态趋势。

## ADDED Requirements

### Requirement: Player trend sparkline in the analysis drawer

系统 SHALL 在展开的选手抽屉里显示跨场走势：

- 选手指最近比赛多于一场时 SHALL 显示一行，含 Rating sparkline、首尾 Rating 差值、窗口内胜场数、ADR sparkline 与首尾 ADR 差值。
- SHALL 追加最近三场的地图、对手、胜负与 Rating 摘要。
- SHALL NOT 在渲染路径同步查询数据库；数据 SHALL 在展开时后台加载并按 SteamID64 缓存。
- 选手仅有单场比赛或数据缺失时 SHALL NOT 显示该行。
- 对手名缺失时 SHALL 显示"未知对手"，地图缺失时显示 `?`。

#### Scenario: Sparkline window

- **WHEN** 选手有 3 场比赛（Rating 1.10 → 1.40 → 0.90，其中 2 胜）
- **THEN** 行内为 `近 3 场 Rating <sparkline> 1.10 → 0.90 (2 胜)` 与 `ADR <sparkline> 90.0 → 70.0`

#### Scenario: Window trimming

- **WHEN** 以窗口 2 渲染三场数据
- **THEN** 展示最近两场，首值为 1.40

#### Scenario: Single match

- **WHEN** 选手只有一场已解析比赛
- **THEN** 不渲染走势行

#### Scenario: Unknown opponent

- **WHEN** 某场数据的对手队名为空
- **THEN** 摘要中以"未知对手"占位而不是空串

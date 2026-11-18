# Spec Delta

## ADDED Requirements

### Requirement: Unclipped Settings & Tasks queue visibility at default window height
The `Settings & Tasks` view SHALL use compact vertical card padding and spacing so `4. 后台渲染任务队列` and its empty-state prompt remain unclipped inside the `1360x840` main window.

#### Scenario: Viewing Settings & Tasks at 1360x840
- **WHEN** the user opens `Settings & Tasks` at the default `1360x840` window size with an empty render queue
- **THEN** the `4. 后台渲染任务队列` header and its empty-state message fit cleanly above the bottom edge without half-line clipping

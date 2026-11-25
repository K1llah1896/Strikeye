# Tasks

- [x] 1.1 `analysis_trend.go`: `loadTrend`（后台 + 去重）、`trendFor`、`trendSummary`、`trendMatchesText`、`minInt`
- [x] 1.2 `View.trends`/`View.trendLoading`、`SelectDemo` 重置、展开行触发加载
- [x] 1.3 抽屉第三行渲染 Rating/ADR sparkline + 最近三场摘要（`len(points) > 1` 才显示）
- [x] 2.1 `analysis_trend_test.go`: sparkline 与差值、窗口裁剪、空数据、对手/地图回退
- [x] 2.2 `go test ./...` 全绿；真实数据 KSCERATO 两场 1.55 → 1.45

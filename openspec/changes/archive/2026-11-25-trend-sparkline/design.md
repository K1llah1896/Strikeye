# Design: 选手跨场走势

## Decisions

1. **复用 `sparkline` 而不是引入图表库**：mygo 没有内建折线图，自己画 canvas 会引入坐标系、刻度、缩放一堆状态；文本 sparkline 已经在本仓库使用（区域控制卡片的控制曲线），一行字符即可读，且天然适配窄面板。
2. **展开时才查询**：`profile.Trend` 要遍历全部 demo 的选手行，放在 `Render` 里会每帧查询一次。改为点击行时触发一次性后台加载，结果缓存在 `View.trends`（按 SteamID64 索引），加载中用 `View.trendLoading` 去重，完成后 `invalidate()` 重绘。
3. **数据不足不显示**：只有一场比赛时"趋势"无意义，`len(points) > 1` 才渲染，避免抽屉里出现孤零零的箭头。
4. **窗口 10 场**：跨场记录可能很长，`trendSummary/trendMatchesText` 都接受 `maxGames` 并取尾部（最近的比赛），保持一行可读。
5. **对手名回退**：老数据可能没有队名，显示"未知对手"而不是空白串，避免出现 `""` 这样的空洞。

## Risks

- 大量 demo 时首次展开有毫秒级查询延迟；已用后台 goroutine + 缓存规避，且失败时只置空不报错。

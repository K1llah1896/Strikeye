# 头像手动刷新

## 为什么

头像只在首次加载时按 30 天 TTL 抓取；玩家换了头像之后，除了删缓存目录没有别的办法更新。资料库/分析页的头像是识别选手最快的视觉线索，值得一个刷新按钮。

## 变什么

1. 分析页「全场选手数据」标题行新增「刷新头像」按钮与状态文案：
   - 后台按 `0` 的 TTL 强制重取全队头像（`avatars.NewClient()`），完成后重新解码并替换视图里的位图；
   - 显示「下载头像...」徽章与「头像：新下载 N，已缓存 M」结果。
2. 顺手修 `internal/avatars.Client.download` 的 nil HTTP 崩溃：零值 `Client`（只设 `ProfileURL`）过去会 panic，现在回退 `http.DefaultClient`。

## 影响

- capability：`player-profiles`（ADDED「Manual avatar refresh」）。
- 无新表、无新依赖。

# zero-click-refresh

## Context

头像与封禁是仅剩的两类“必须联网、此前需要点按钮”的数据。两者都已有 TTL 缓存与磁盘
布局（`<DataRoot>/cache/radar_cards/portraits`、`player_bans` 表），因此自动化只是把
已有的手动入口接到视图生命周期上，不需要新的网络层。

约束：

- 渲染测试与 CI 不能依赖 Steam；必须可以关掉自动抓取、也必须能注入假客户端。
- 没有 API key 时封禁刷新一次请求也不能发（key 是可选配置）。
- 自动路径不能覆盖用户手动设置的档案字段（姓名、备注、标签），与 `profile.Save`
  的“只补空”语义一致。

## Decisions

- **头像在 `SelectDemo` 之后触发**，而不是在资料库列表里：列表可能有上千场，逐个抓会
  把 Steam 打成限流；打开一场比赛是用户唯一的动作，10 个选手一次补齐。
- `ensureAvatarsAsync` 只处理缓存缺失的选手，`avatars.Fetch` 内部已有 30 天 TTL，重复
  打开同一场是零请求。
- 新增 `View.avatarAutoFetch`（`NewView` 置 true）与 `View.avatarClient`（nil 表示真实
  客户端）。渲染测试设为 false，下载路径由 `TestEnsureAvatarsAsyncFetchesMissing` 用
  `httptest` 服务假的 `?xml=1` 响应验证。
- **封禁刷新挂在资料库 `Refresh` 之后**，只在 `loadSteamAPIKey` 非空时启动，并用
  `refreshBans(ctx, db, key, false)`——`refresh=false` 会让 `vac.Service.Lookup` 只对
  缺失或超过 24 小时 TTL 的选手发请求，日常刷新零网络。
- 用 `bansBusy` 防止 `Refresh` 被频繁调用时叠加 goroutine；完成后重新 `Refresh` 让徽章
  立即更新。
- 手动按钮保留：头像「刷新头像」用 `maxAge=0` 强制重取，封禁盾牌按钮用 `refresh=true`。

## Risks

- 头像自动抓取会在用户打开 demo 时访问 `steamcommunity.com`；失败静默（沿用原行为，
  表内回退状态点），不影响任何分析功能。
- 封禁自动刷新依赖 key 存在；没有 key 时徽章仍读缓存行，与之前一致。

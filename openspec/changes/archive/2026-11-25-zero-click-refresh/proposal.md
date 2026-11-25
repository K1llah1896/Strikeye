# zero-click-refresh

## Why

内置地图数据与工具已经随包分发，但还有两类数据必须联网、此前只能靠用户点按钮拿到：
选手头像（分析页「刷新头像」）与封禁状态（资料库盾牌按钮）。用户明确要求“不要让用户去
点击下载”，所以这两条也改成打开即自动补齐：有缓存就不动，缓存过期才请求。

## What Changes

- 分析页 `SelectDemo` 后异步补齐缺失头像：只对缓存里没有的选手发请求，写盘后重新解码
  位图并重绘；新增 `View.avatarAutoFetch`（默认开，渲染测试关闭）与可注入的
  `View.avatarClient`，`ensureAvatarsAsync` 的下载路径由 `httptest` 覆盖。
- 资料库 `Refresh` 后异步刷新过期封禁快照：仅当存在 Steam Web API key 时启动，复用
  `vac.Service.Lookup(refresh=false)` 的 24 小时 TTL，未过期选手零请求；无 key 完全不启动。
- 保留手动按钮（「刷新头像」「封禁」）作为强制刷新手段。

## Impact

- `player-profiles`、`account-risk` 两个能力各新增一条要求。
- 测试新增 `TestEnsureAvatarsAsyncFetchesMissing`、`TestEnsureAvatarsAsyncRespectsDisabled`、
  `TestEnsureBansAsyncWithoutKeyIsNoop`；渲染测试显式关闭自动抓取以保持无网络。

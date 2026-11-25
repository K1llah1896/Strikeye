# player-profiles

## ADDED Requirements

### Requirement: Manual avatar refresh

分析页 SHALL 支持手动刷新选手头像：

- 「全场选手数据」标题行 SHALL 提供「刷新头像」按钮；进行中 SHALL 显示徽章并阻止重复触发。
- 刷新 SHALL 忽略 30 天 TTL 重取全队头像，完成后重新解码并更新表格位图。
- 完成后 SHALL 显示「头像：新下载 N，已缓存 M」。
- `avatars.Client` 的零值（未设置 `HTTP`）SHALL 回退到 `http.DefaultClient`，不得 panic。

#### Scenario: Refresh roster avatars

- **WHEN** 用户点击「刷新头像」
- **THEN** 位图按新下载结果更新，标题行显示新下载与已缓存数量

#### Scenario: Zero client

- **WHEN** 调用方用只设置 `ProfileURL` 的 `avatars.Client{}` 发起请求
- **THEN** 请求使用默认 HTTP 客户端并返回网络错误，而不是崩溃

# account-risk delta

## ADDED Requirements

### Requirement: Ban snapshots refresh automatically

资料库刷新时 MUST 自动更新过期的封禁快照，不要求用户点击按钮。

#### Scenario: Stale snapshots are refreshed on library refresh

- **WHEN** 已配置 Steam Web API key，且资料库刷新时存在缺少快照或快照超过 24 小时的选手
- **THEN** 视图在后台查询这些选手的封禁状态并写库，随后刷新资料库使徽章更新
- **AND** 24 小时内的快照不产生网络请求

#### Scenario: No key means no requests

- **WHEN** 没有配置 API key
- **THEN** 自动刷新完全不启动，徽章继续读取既有缓存行

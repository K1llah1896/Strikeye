# account-risk

## ADDED Requirements

### Requirement: In-app ban refresh

资料库 SHALL 提供封禁刷新入口：

- 工具栏「刷新封禁」SHALL 查询全部已解析比赛的去重选手，后台执行并回报检查数、刷新数、跳过数与封禁名单。
- 有 Key 时 SHALL 请求 Steam `ISteamUser/GetPlayerBans` 并写入 `player_bans`；无 Key 时 SHALL 只读取缓存行，未缓存的计入 `skipped`。
- Key SHALL 保存在 `<DataRoot>/cache/steam_api_key.txt`（0600）；`STRIKEYE_STEAM_API_KEY` 环境变量 SHALL 覆盖文件值。
- 缺少 Key 时 SHALL 弹出 Key 面板，保存后立即执行一次查询。
- 查询 SHALL 受超时约束（90 秒），失败 SHALL 在状态栏显示原始错误。
- 刷新成功后 SHALL 重新加载资料库，使「封禁 N」徽章立即反映新数据。

#### Scenario: Refresh with key

- **WHEN** 已保存有效 Key，用户点击「刷新封禁」
- **THEN** 状态栏显示已检查人数与封禁名单，卡片徽章更新

#### Scenario: No key yet

- **WHEN** 没有 Key 时点击「刷新封禁」
- **THEN** 显示 Key 输入面板而不是报错；保存后立即查询

#### Scenario: Offline cached rows

- **WHEN** 没有 Key 但 `player_bans` 已有历史记录
- **THEN** 已知封禁仍被计入封禁名单，未缓存选手计入跳过

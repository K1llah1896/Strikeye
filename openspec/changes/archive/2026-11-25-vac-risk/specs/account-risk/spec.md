# account-risk

## ADDED Requirements

### Requirement: Steam ban snapshot cache

系统 SHALL 把 Steam 封禁状态缓存到 SQLite：

- SHALL 在迁移 v6 建 `player_bans` 表，字段包含 `vac_banned`、`number_of_vac_bans`、`number_of_game_bans`、`community_banned`、`days_since_last_ban`、`economy_ban`、`checked_at`。
- SHALL 提供 `Upsert`/`Get`/`List`/`BannedFor`；`BannedFor` SHALL 用单条 `IN (...)` 查询返回有封禁的 SteamID 集合。
- 缓存 SHALL 默认 24 小时有效；超过 TTL 或显式 `--refresh` 时才重新请求 Steam。
- Steam 未返回某账号时 SHALL 标记 `skipped`，不得写入伪造的"干净"记录。
- 未配置 API key 时 SHALL 只读缓存并对未缓存账号返回 `skipped`，且不得报错退出。

#### Scenario: Cache hit

- **WHEN** 一小时内第二次查询同一批 SteamID
- **THEN** 结果为 `cache=hit`，且不发出 HTTP 请求

#### Scenario: Stale entry refreshes

- **WHEN** 缓存快照超过 24 小时
- **THEN** 下一次查询重新请求 Steam 并更新 `checked_at`

#### Scenario: Missing key

- **WHEN** 没有 API key 且账号从未缓存
- **THEN** 该账号返回 `cache=skipped`、`at_risk=0`，命令退出码为 0

### Requirement: Ban lookup client

`internal/vac` SHALL 通过 `ISteamUser/GetPlayerBans/v1` 查询封禁：

- SHALL 以逗号拼接 SteamID 一次请求，带 `key` 与 `steamids` 查询参数。
- HTTP 非 200 SHALL 返回错误并截断响应体到 160 字符。
- 响应体读取 SHALL 限制在 1MB，HTTP 超时 SHALL 为 15 秒。
- `Ban` SHALL 提供 `Risk()` 与 `RiskLabel()`，后者优先级为 VAC > 社区 > 游戏封禁 > 无封禁记录。

#### Scenario: Parse bans

- **WHEN** Steam 返回一个 VAC 封禁账号与一个社区封禁账号
- **THEN** 两者的 `RiskLabel()` 分别为「VAC 封禁」与「社区封禁」

#### Scenario: HTTP failure

- **WHEN** Steam 返回 403
- **THEN** 返回带状态码与响应片段的错误

### Requirement: Ban surface in CLI and library

`strikeye-cli bans` SHALL 输出 `checked/from_cache/skipped/at_risk/has_steam_key/bans`，并在未给 `--steamid` 时默认覆盖 `--demo-id` 指定场次的全部选手。

资料库卡片 SHALL 对存在缓存封禁记录的场次显示「封禁 N」徽章；`N=0` 时 SHALL NOT 显示徽章。

`strikeye-cli profile` 的报告 SHALL 在有缓存时附带 `bans` 字段，无缓存时省略。

#### Scenario: Demo wide lookup

- **WHEN** 执行 `bans --data-root dist` 且没有 key
- **THEN** 输出 `checked=10`、`skipped=10`、`has_steam_key=false`

#### Scenario: Library badge

- **WHEN** 某场次的两名选手有缓存封禁记录
- **THEN** 该卡片显示「封禁 2」

#### Scenario: Profile includes snapshot

- **WHEN** 某选手已有封禁快照且执行 `profile --steamid`
- **THEN** 输出包含 `bans.vac_banned` 与 `bans.checked_at`

## Why

报告里「跨比赛玩家档案 / 标签 / VAC」这一格目前只有档案与标签，没有封禁信息。判断对手是否可疑、复盘自己打过的人是否后来被封，都需要 Steam 的封禁状态；CS:DM 也把 VAC 追踪当作基础能力。

Steam 只在 `ISteamUser/GetPlayerBans` 暴露该数据且必须带 API key，因此本 change 的做法是：**有 key 就查、没 key 只用缓存**，把"未检查"与"干净"明确区分开。

## What Changes

- 迁移 v6：`player_bans` 表（VAC/社区/游戏封禁、距上次封禁天数、economy ban、`checked_at`）+ `checked_at` 索引。
- 新增 `internal/vac`：`Client`（Steam Web API，15s 超时、1MB 响应上限）、`Ban` + `Risk()`/`RiskLabel()`、`Service`（SQLite 缓存，默认 TTL 24h，`--refresh` 强制刷新）。
- 新增 CLI `bans`：`--demo-id`（默认覆盖该场全部选手）、`--steamid a,b`、`--refresh`、`--api-key`（缺省读 `$STRIKEYE_STEAM_API_KEY`）。
- 资料库卡片新增「封禁 N」红色徽章（只统计已缓存且有封禁的账号）。
- `strikeye-cli profile` 的报告新增 `bans` 字段（有缓存才出现）。

## Impact

- 新增：`internal/vac`、`internal/storage/player_bans_repo.go`、`cmd/strikeye-cli/bans.go`。
- 迁移：v6（追加表，不改旧表）。
- 资料库刷新多一次 `BannedFor` 查询（按 SteamID 集合，单条 SQL）。
- 无网络/无 key 时功能可用但只读缓存，UI 不会伪造"干净"结论。

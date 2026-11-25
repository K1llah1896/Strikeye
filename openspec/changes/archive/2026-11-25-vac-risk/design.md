## Context

Steam Web API 的 `ISteamUser/GetPlayerBans/v1` 一次最多接受 100 个 SteamID，返回 `CommunityBanned/VACBanned/NumberOfVACBans/DaysSinceLastBan/NumberOfGameBans/EconomyBan`。该端点要求 key，且限流严格（约每分钟数次），所以必须缓存。

仓库已有 `player_profiles`（v3）与 `player_bans` 需求的字段高度相似，但语义不同：档案是用户编辑的数据，封禁是外部快照，两者必须分开，避免刷新封禁时覆盖用户备注。

## Goals / Non-Goals

**Goals**

- 一次查询覆盖整场 10 名选手。
- 缓存优先：TTL 内不联网，`--refresh` 才强制。
- 无 key 时仍能读缓存，并把未缓存的账号标成 `skipped`。
- 封禁结论能被资料库与档案复用，不重复请求。

**Non-Goals**

- 不做自动轮询、不做后台定时刷新（限流下会自伤）。
- 不做"作弊判定"：封禁状态只是事实，不是结论。
- 不实现 Steam 登录或需要 cookie 的接口。

## Decisions

1. **迁移 v6 独立表**：与 `player_profiles` 解耦；`checked_at` 让 UI 能显示快照新鲜度。
2. **`RiskLabel()` 单一优先级**：VAC > 社区 > 游戏封禁 > 无记录，避免同一账号出现两个互相矛盾的标签。
3. **`Cache` 字段用 `hit/refreshed/skipped/miss`**：CLI 与测试都能直接断言"这次到底联没联网"。
4. **key 只从 flag 或环境变量读**：不落库、不写日志、不进 JSON 输出（只输出 `has_steam_key`）。
5. **`BannedFor` 用单条 `IN (...)` 查询**：资料库 200 场卡片不会被 N+1 查询拖垮。
6. **无 key 不报错**：`Lookup` 返回缓存 + `skipped`，让"先爬一遍、以后有 key 再刷新"的工作流成立。

## Risks / Trade-offs

- API key 泄漏风险：只经内存传递；建议放在环境变量而非命令行历史。
- Steam 限流：默认 TTL 24h；`--refresh` 会立刻消耗配额。
- 封禁会撤销/误封：`DaysSinceLastBan` 与 `checked_at` 一起展示，避免把旧快照当现状。

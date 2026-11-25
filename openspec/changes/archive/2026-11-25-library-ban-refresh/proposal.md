# 资料库内封禁刷新

## 为什么

资料库卡片已经有「封禁 N」徽章，但徽章只反映上一次 CLI `bans` 写入的缓存；没有 Key 的用户在 GUI 里既看不到也无法更新。封禁信息是"该不该信这场数据"的前置判断，入口必须在 GUI。

## 变什么

1. 资料库工具栏新增「刷新封禁」按钮：
   - 有 Key 时后台查询全部已解析比赛的去重选手，状态栏回报"已检查 N 名 / 刷新 X / 跳过 Y"以及被封禁者名单；
   - 没有 Key 时弹出 Key 面板（不静默失败）。
2. Steam Web API Key 存 `<DataRoot>/cache/steam_api_key.txt`（0600），`STRIKEYE_STEAM_API_KEY` 环境变量优先；不写入 `config.json`。
3. 无 Key 时仍可读缓存行：已缓存的封禁判定照常生效，未缓存的标记 `skipped`（与 CLI 一致）。

## 影响

- capability：`account-risk`（ADDED「In-app ban refresh」）。
- 复用 `internal/vac` 的 `Service.Lookup` 与 `player_bans` 表，无新表。

# 中文检索子串匹配

## 为什么

FTS5 的 `unicode61` 分词器把整段中文当成一个 token：备注 `LongA 默认双架` 入索引后是一个 token，用户搜"双架"得到 0 条结果。实测 `strikeye-cli search --data-root dist --query 双架` 返回 `count: 0`，而数据里明明有这条备注——中文用户的关键词搜索等于失效。

## 变什么

1. `SearchRepo.Query` 在 FTS 之外增加子串兜底：
   - 查询含非 ASCII 字符时，额外跑一次 `lower(col) LIKE '%kw%'`（覆盖 players/teams/map_name/note），与 FTS 结果按 `demo_id` 去重合并；
   - 纯 ASCII 查询保持原 FTS 路径不变（性能与排序都不受影响）。
2. 空查询（FTS 词与子串词都为空）直接返回空，不发起 SQL。

## 影响

- capability：`sqlite-storage-and-migration`（ADDED「CJK substring search fallback」）。
- 无迁移、无新索引；LIKE 扫描对上千行规模无压力（已有 2,000 行压测基线）。

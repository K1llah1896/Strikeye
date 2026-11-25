# Design: 资料库全文检索

## Context

SQLite（modernc）自带 FTS5，但 `match_search` 的文本来自多张表：队伍与地图在 `demos`，选手在 `match_players`，备注来自 `demos.remark` 与 `map_annotations.note`。这决定了索引必须由我们拼装。

## Decisions

1. **整库重建而非增量触发器**：资料库规模是"千级"，一次重建 2000 条约 15ms；SQLite 触发器要覆盖 demos/match_players/map_annotations 三张表的增删改，维护成本远高于收益。`EnsureSearchIndex` 用"索引行数 ≠ demo 数"作为失效信号，覆盖了新增/删除；文本编辑（备注、标记）由写路径显式重建。
2. **前缀匹配 + bm25**：用户键入的是半个词（"ksce"），因此每个 token 追加 `*`；排序交给 `bm25()`，同一关键词命中的 demo 按相关度排列。
3. **token 白名单而非转义**：FTS5 的引号转义规则容易漏（`"ksce"*` 这种写法会被拒绝）。把输入按非字母数字切开、丢掉 `and/or/not/near` 保留字后，剩下的 token 不可能构成操作符，前缀 `*` 也就永远合法。代价是短语查询（带空格的精确短语）退化为 AND 前缀查询——对搜索框足够。
4. **UI 用并集而不是替换**：内存匹配包含文件名与标签等 FTS 没收录的字段，FTS 包含备注文本。两者求并集才能"输入什么都能找到"，同时不牺牲即时性（内存匹配当帧生效，FTS 结果下一帧并入）。
5. **`UNINDEXED demo_id` + 外部 join**：FTS 表只存文本与 demo id，不复制 demo 元数据；渲染时按 id 去主表取。

## Risks

- `EnsureSearchIndex` 用行数当失效信号，如果备注/标记被外部工具改动而 demo 数不变，索引会滞后；`search --reindex` 与写路径重建兜底。
- CJK 文本依赖 `unicode61` 的连续字符串切分，跨标点的中文短语查不到；这是 FTS5 分词的固有边界，记录在方案里而不是假装支持。

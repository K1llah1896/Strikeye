# Spec Delta

## ADDED Requirements

### Requirement: Library full-text index

系统 SHALL 维护 FTS5 全文索引：

- 迁移 v10 SHALL 建立 `match_search(demo_id UNINDEXED, players, teams, map_name, note)` 使用 `unicode61` 分词。
- 重建 SHALL 从 `demos`（队伍、地图、备注）、`match_players`（选手名）与 `map_annotations`（callout 与备注）汇总文本。
- `EnsureSearchIndex` SHALL 在索引行数与 demo 数不一致时重建，并报告是否重建。
- `Query(ctx, text, limit)` SHALL 对每个 token 做前缀匹配、按 `bm25` 升序返回、遵守 limit（默认 50、上限 500）。
- 查询构造 SHALL 只保留字母数字 token 并丢弃 FTS5 保留字，因此用户输入的操作符与引号 SHALL NOT 造成 SQL 语法错误。
- 空查询 SHALL 返回零结果而不访问数据库。

#### Scenario: Player prefix match
- **WHEN** the index contains player `KSCERATO` and the query is `ksce`
- **THEN** that demo is returned

#### Scenario: Operators cannot break the query
- **WHEN** the query is `NiKo" OR NEAR(`
- **THEN** the query succeeds and behaves like a two-token AND prefix search

#### Scenario: Stale index is repaired
- **WHEN** a demo is added after the last rebuild
- **THEN** `EnsureSearchIndex` reports a rebuild and the new demo becomes searchable

### Requirement: Large library pagination

系统 SHALL 支持大资料库分页读取：

- `ListPage(ctx, limit, offset)` SHALL 使用与 `List` 相同的排序，limit 上限 1000、默认 100，负 offset 归零。
- `CountDemos` SHALL 返回 demo 总数。
- 测试 SHALL 覆盖 2000 条 demo 下的分页、索引重建与查询耗时上限（分页 <200ms、FTS 查询 <100ms）。

#### Scenario: Page offset beyond the first page
- **WHEN** requesting `ListPage(50, 950)` on a 2000-row library
- **THEN** 50 rows are returned within the time budget

# Tasks

- [x] 1.1 迁移 v10 `match_search`（FTS5 / unicode61 / demo_id UNINDEXED）
- [x] 1.2 `SearchRepo`：`RebuildSearchIndex`、`EnsureSearchIndex`、`Query`、`ftsQuery`（token 白名单 + 保留字过滤 + 前缀 `*`）
- [x] 1.3 `DemoRepository.ListPage`、`CountDemos`
- [x] 1.4 CLI `search --query [--limit] [--reindex]`
- [x] 1.5 资料库搜索接入 FTS 并集（`ensureFullTextSearch`、`FilterCriteria.ExtraDemoIDs`）
- [x] 2.1 测试：前缀/地图/备注/标记备注/操作符注入/空查询/limit、2300 行规模下的分页与索引、UI 并集与异步刷新
- [x] 2.2 benchmark：`BenchmarkDemoListPage`、`BenchmarkSearchQuery`
- [x] 2.3 `go test ./...` 全绿；`openspec validate --all --strict` 通过

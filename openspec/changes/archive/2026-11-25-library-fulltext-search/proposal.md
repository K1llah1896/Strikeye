# Proposal: 资料库全文检索与大库分页

## Why

资料库的搜索一直是"把全部 demo 读进内存再逐字段 `strings.Contains`"。这对当前两个 demo 够用，但报告 §4.10 把"数据库演进"列为 P1 缺口时点到了两个具体问题：没有 FTS5 全文检索，也没有大库分页与压测。字段匹配还漏掉了注释类文本：战术标记的备注（`map_annotations.note`）从来没有被搜索过。

## What Changes

- 迁移 v10 建立 FTS5 虚拟表 `match_search(demo_id UNINDEXED, players, teams, map_name, note)`。
- `storage.SearchRepo`：`RebuildSearchIndex`（从 demos/match_players/map_annotations 重建）、`EnsureSearchIndex`（行数不一致时自动重建）、`Query`（前缀匹配 + `bm25` 排序 + limit）。
- 查询构造器 `ftsQuery`：把用户输入切成纯字母数字 token 再追加 `*`，并丢弃 FTS5 保留字，避免用户输入 `OR`/`NEAR(`/引号时崩成语法错误。
- `DemoRepository.ListPage(ctx, limit, offset)` 与 `CountDemos`：大库分页读取。
- CLI `search --query <text> [--limit] [--reindex]`。
- 资料库搜索框接入 FTS：关键词变化时后台查一次索引，命中集合与内存匹配取并集（因此可以搜到战术标记备注）。
- 压测：2000 条 demo 的分页/重建/查询断言与 Go benchmark。

## Impact

- Affected specs: `match-library-and-analysis-ui`（搜索覆盖 FTS 命中）、`sqlite-storage-and-migration`（迁移 v10 与 FTS）
- Affected code: `internal/storage/{migrations.go,search_repo.go,demo_repo.go}`、`internal/ui/views/library/library.go`、`cmd/strikeye-cli/search.go`

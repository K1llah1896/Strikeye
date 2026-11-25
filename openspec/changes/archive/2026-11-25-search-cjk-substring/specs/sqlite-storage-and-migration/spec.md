# sqlite-storage-and-migration

## ADDED Requirements

### Requirement: CJK substring search fallback

资料库全文检索 SHALL 支持中文子串查询：

- 查询含非 ASCII 字符时，除 FTS5 前缀匹配外 SHALL 额外执行大小写不敏感的子串匹配（`players`/`teams`/`map_name`/`note`）。
- 两路结果 SHALL 按 `demo_id` 去重合并，FTS 命中优先保留其 bm25 排序。
- 纯 ASCII 查询 SHALL 只走 FTS5，不改变既有排序。
- 空查询 SHALL 返回空结果且不执行 SQL。

#### Scenario: Search a Chinese note

- **WHEN** 备注为 `LongA 默认双架`，用户搜索「双架」
- **THEN** 该比赛出现在结果中，`note` 字段包含完整备注

#### Scenario: Absent Chinese term

- **WHEN** 用户搜索一个中文词库里不存在
- **THEN** 返回 0 条，不报错

#### Scenario: Latin query unchanged

- **WHEN** 用户搜索 `furia`
- **THEN** 走 FTS5 前缀匹配，结果按 bm25 排序

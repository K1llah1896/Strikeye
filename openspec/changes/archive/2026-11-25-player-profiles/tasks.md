## 1. 存储与迁移

- [x] 1.1 迁移 v3 新增 `player_profiles`（SteamID64 主键、display_name、note、tags_json、updated_at）
- [x] 1.2 `PlayerProfileRepository` 提供 Upsert/Get/List/Delete/TagsForSteamIDs
- [x] 1.3 标签归一化与中英文逗号/分号解析（`NormalizeTags`、`ParseTagList`）

## 2. 档案聚合与 CLI

- [x] 2.1 `internal/profile.Build` 复用 `export.AggregatePlayer`，显示名覆盖仅影响展示
- [x] 2.2 `profile.Save` 合并语义：仅覆盖传入字段，标签为空则保留
- [x] 2.3 `strikeye-cli profile --steamid [--name --note --tags]` 输出档案或保存结果

## 3. 比赛库展示

- [x] 3.1 `Refresh` 按 Demo 批量取标签并入 `MatchEntry.PlayerTags`
- [x] 3.2 卡片最多渲染 3 个标签胶囊
- [x] 3.3 关键词过滤同时匹配选手标签

## 4. 验证

- [x] 4.1 存储单测覆盖归一化、覆盖写入、缺失查询、列表、IN 查询、删除
- [x] 4.2 profile 单测覆盖无档案/有档案/合并/无比赛错误
- [x] 4.3 CLI 与 library 单测覆盖保存、查看、标签过滤与刷新
- [x] 4.4 真实双图数据验证：KSCERATO 两场聚合 + 标签写入/读取
- [x] 4.5 `go test ./...` 全绿；`openspec validate player-profiles --strict` 通过

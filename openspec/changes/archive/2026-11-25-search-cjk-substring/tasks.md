# Tasks

- [x] `Query` 拆分为 `queryFTS` + `querySubstring`，按 `demo_id` 去重合并
- [x] `needsSubstringPass`（含非 ASCII 才走 LIKE）
- [x] 测试：中文备注命中、缺失中文不误命中、拉丁词仍走 FTS
- [x] 真实数据验证 `strikeye-cli search --query 双架` 命中 id2
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 报告 §8/§4.10 更新

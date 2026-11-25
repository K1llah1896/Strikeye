# Tasks

- [x] `internal/export/batch.go`：`BatchFilter`、`BatchItem`、`BatchResult`、`Batch`、`describeFilter`、`writeBatchIndex`
- [x] CLI：`export --all --team --map --limit`，`runExportBatch` 分派，`--all` 与单场选择互斥
- [x] 测试：`internal/export/batch_test.go`（目录隔离/过滤/limit/空选集）、`cmd/strikeye-cli/export_batch_test.go`（`--format all` 四格式 + 索引 + 互斥报错）
- [x] 真实数据跑通 `export --all`
- [x] `openspec validate --strict`

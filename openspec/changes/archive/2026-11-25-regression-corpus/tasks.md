# Tasks

- [x] 1.1 `internal/corpus`：`Snapshot`/`PlayerEntry`/`Build`/`BuildWithGeometry`/`Diff`/`ReadSnapshot`/`WriteSnapshot`/`copyGeometry`
- [x] 1.2 CLI `corpus`（`--dir --golden --demo --update --geometry-dir --work-dir`，失败非零退出）
- [x] 1.3 生成 `tests/fixtures/golden/*.json`（含几何后的 TTD 样本）
- [x] 2.1 测试：`Diff` 容差与差异、快照往返、`STRIKEYE_CORPUS=1` opt-in 的 fixture 比对
- [x] 2.2 真实运行：`corpus --update` 44s 生成两图快照；`corpus` 复跑 0 差异
- [x] 2.3 `go test ./...` 全绿；`openspec validate --all --strict` 通过

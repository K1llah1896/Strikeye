# Proposal: 多来源回归语料与指标黄金快照

## Why

报告 §4.9「工程底座」把「多来源回归语料 + 黄金指标快照」列为 P1 缺口：仓库只有两份 HLTV fixture 的端到端测试，但它们只断言"解析不报错"，没有锁住指标数值。任何解析器或指标改动（例如把 `team_a`/`team_b` 判反、把 TTD 窗口改坏）都能悄悄通过 CI。

## What Changes

- 新增 `internal/corpus`：把一次解析压缩成 `Snapshot`（身份、结构计数、逐选手指标），提供 `BuildWithGeometry`、`Diff`（浮点容差 0.11）、`ReadSnapshot`/`WriteSnapshot`。
- 黄金快照 `tests/fixtures/golden/<fixture>.json`，覆盖 map/队伍/比分、回合与事件数、回放弹道与死亡数、购买笔数，以及 10 名选手的 kills/deaths/assists/HS/ADR/KAST/Rating/TTK/TTD/首杀。
- CLI `corpus [--dir] [--golden] [--demo] [--update] [--geometry-dir] [--work-dir]`：`--update` 重写快照，否则逐项比对并在不一致时非零退出。
- 几何目录可选：传入后把 `.tri` 复制进临时缓存，使 TTD 指标也进入快照（否则 TTD 恒为 0，等于没测）。
- Go 测试 `TestFixtureCorpus` 以 `STRIKEYE_CORPUS=1` opt-in，避免 30 秒解析拖慢常规 `go test`。

## Impact

- Affected specs: `metric-snapshots`（新增）
- Affected code: `internal/corpus/`、`cmd/strikeye-cli/corpus.go`、`tests/fixtures/golden/`

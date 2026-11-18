# Tasks

## 1. SQLite Schema v2 & Storage Repository

- [x] 1.1 Add Migration v2 (`hltv_match_analysis_v2`) in `internal/storage/migrations.go` extending `match_players` (`flash_assists`, `enemies_flashed`, `total_flash_sec`, `round_swing`, `eco_adj_rating`, `eco_adj_adr`, `clutch_breakdown_json`) and creating `match_player_sides`; verify with `go test ./internal/storage/...` covering v1-to-v2 upgrade and automatic pre-migration snapshot.
- [x] 1.2 Extend `MatchPlayer` and add `MatchPlayerSide` CRUD methods (`ReplaceMatchPlayerSides`, `ListMatchPlayerSides`) in `internal/storage/analysis_repo.go`, ensuring legacy v1 demos return empty side slices without error; verify with unit tests in `internal/storage/storage_test.go`.

## 2. Single-Pass Parser & Metrics Engine

- [x] 2.1 Hook `events.PlayerFlashed` in `internal/parser/single_pass.go` to track enemy flash counts and blind duration (excluding self/team flashes) and wire `IsAssistedFlash` flash assists; verify with parser unit tests in `internal/parser/single_pass_test.go`.
- [x] 2.2 Extend `ComputeMetrics` in `internal/parser/metrics.go` to output `CT`/`T` side-split stats, granular `1v1`–`1v5` clutch breakdowns, `Round Swing` win-probability delta, and `Eco-Adjusted` Rating/ADR; verify with deterministic test cases in `internal/parser/metrics_test.go`.
- [x] 2.3 Persist v2 player columns and `match_player_sides` in `internal/parser/single_pass.go` and bump `ParserBaselineVersion` in `internal/version/parser_baseline.go`; verify `go test ./internal/parser/...` passes.

## 3. Match Analysis UI (6-Tab HLTV Upgrade)

- [x] 3.1 Update `TabOverview` in `internal/ui/views/analysis/analysis.go` to add `All | CT | T` side-split toggle, `Eco-Adjusted` toggle, sortable `Round Swing` column, and expandable player detail drawer (`2K`–`5K`, `1v1`–`1v5`, `Utility ADR`, `Opening%`, `AWP Kills`, flash stats), degrading gracefully for v1 demos; verify with `go test ./internal/ui/...`.
- [x] 3.2 Upgrade `TabEconomy` in `internal/ui/views/analysis/analysis.go` with Economy & Tactical KPI summary cards (Pistol, Conversion, `Full vs Full`, `Force/Semi vs Full`, `Eco vs Full`, A/B Post-Plant Win%), halftime divider, dynamic per-round `CT`/`T` labels, and clickable round expansion (10-player `player_economy_json`, round kill feed, and `Jump to 2D Replay` button); verify with unit/UI tests in `internal/ui/views/analysis/analysis_test.go`.
- [x] 3.3 Add the sixth tab (`TabPerformance`: `3. 对位与武器表现`) in `internal/ui/views/analysis/analysis.go` computing the 5v5 H2H Kill Matrix, Opening Duel Matrix, and Weapon Kill Distribution from `match_events`, and shift Radar/2D Replay/Heatmaps to tabs 4–6; verify with `go test ./internal/ui/views/analysis/...` and `go test ./internal/ui/...`.

## 4. End-to-End Verification

- [x] 4.1 Run full Go test suite (`go test ./...`) and `openspec validate hltv-match-analysis-v2` to confirm all storage, parser, UI, and spec checks pass with zero regressions.

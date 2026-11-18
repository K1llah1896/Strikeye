# Proposal

## Why

Strikeye's Match Analysis workspace already excels at 32Hz 2D tactical replay, KDE heatmaps, and 2560x1440 radar cards, but its match-level statistical depth lags behind modern HLTV match analytics (e.g., Rating 3.0 / Round Swing / Eco-Adjusted metrics, CT/T side splits, 5v5 head-to-head duel matrices, and round-by-round economy breakdowns). Upgrading the parser, storage schema, and Match Analysis UI to adopt HLTV-style match analysis workflows—while excluding multi-demo BO3/BO5 series aggregation—gives players and analysts complete tactical and statistical visibility from a single `.dem` file.

## What Changes

- **Expand Match Analysis UI from 5 tabs to 6 tabs** (`internal/ui/views/analysis/analysis.go`):
  - **Tab 1 (`1. 概览与记分板`)**: Add `All | CT | T` side-split filter toggle, `Eco-Adjusted` rating/ADR toggle, and `Round Swing` column alongside existing `Strikeye Rating`; add clickable player detail drawer exposing `2K/3K/4K/5K` multi-kill breakdown, `1v1`–`1v5` clutch breakdown, `Utility ADR`, `Opening Duel Win%`, `AWP Kills`, and flash utility stats (`Flash Assists`, `Enemies Flashed`, `Total Blind Sec`).
  - **Tab 2 (`2. 回合与经济走势`)**: Add top Economy & Tactical KPI summary cards (Pistol Win%, Post-Pistol Conversion%, `Full vs Full`, `Force/Semi vs Full`, `Eco vs Full`, and A/B site Post-Plant Win%); add halftime divider (`1st Half | 2nd Half | OT`) and dynamic per-round `CT`/`T` side labels (replacing hardcoded `TEAM A (CT)`/`TEAM B (T)`); support clicking a round to inspect 10-player `player_economy_json` equipment/spend details, round kill feed, and a "Jump to 2D Replay" button.
  - **Tab 3 (`3. 对位与武器表现`)** *(new dedicated tab)*: Display 5v5 Head-to-Head (H2H) Kill Matrix, Opening Duel Matrix, and match-wide Weapon Kill Distribution computed from `match_events`.
  - **Tabs 4–6**: Preserve `Players & CS Data Radar`, `2D Replay`, and `Heatmaps` as tabs 4, 5, and 6, wired to accept round jumps from Tab 2.
- **Enhance Single-Pass Parser (`internal/parser/`)**:
  - Hook `events.PlayerFlashed` in `single_pass.go` to collect `EnemiesFlashed`, `FlashAssists`, and `TotalFlashSec` (excluding self/team flashes).
  - Compute `ALL`, `CT`, and `T` side-split player statistics in `metrics.go`.
  - Persist granular `1v1`–`1v5` clutch attempts/wins per player.
  - Compute `Round Swing` (win-probability delta per kill/clutch based on alive-count differential and bomb state) and `Eco-Adjusted` Rating/ADR alongside the baseline `Strikeye Rating`.
- **Add SQLite Schema Migration v2 (`internal/storage/`)**:
  - Extend `match_players` with `flash_assists`, `enemies_flashed`, `total_flash_sec`, `round_swing`, `eco_adj_rating`, `eco_adj_adr`, and `clutch_breakdown_json`.
  - Add `match_player_sides` table keyed by `(demo_id, steam_id64, side)` for `CT` and `T` split metrics, keeping `match_players` backwards-compatible.
  - Ensure older demos parsed under schema/parser v1 degrade gracefully in the UI without crashes or re-parse loops.

## Capabilities

### New Capabilities
<!-- None; all changes extend existing cohesive capabilities -->

### Modified Capabilities
- `match-library-and-analysis-ui`: Expand Match Analysis from 5 tabs to 6 tabs (adding `Performance` H2H/weapon tab), add `All/CT/T` side-split & `Eco-Adjusted` toggles, player detail drawer, round-level economy/kill-feed inspection with 2D Replay jump, and dynamic halftime/side labels.
- `single-pass-parser`: Extract `PlayerFlashed` metrics, `CT`/`T` side-split stats, `1v1`–`1v5` clutch breakdowns, `Round Swing`, and `Eco-Adjusted` Rating/ADR during single-pass parsing.
- `sqlite-storage-and-migration`: Add schema migration v2 with `match_player_sides` table and extended `match_players` columns while preserving pre-migration snapshots and backwards compatibility for v1-parsed demos.

## Impact

- `internal/parser/single_pass.go`, `internal/parser/metrics.go`, `internal/parser/metrics_test.go`, `internal/parser/single_pass_test.go`
- `internal/storage/migrations.go`, `internal/storage/analysis_repo.go`, `internal/storage/storage_test.go`
- `internal/ui/views/analysis/analysis.go`, `internal/ui/views/analysis/analysis_test.go`
- `internal/version/parser_baseline.go` (bump parser baseline version so new parses/re-parses are identifiable)

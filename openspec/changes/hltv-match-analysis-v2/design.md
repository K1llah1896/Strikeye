# Design

## Context

Strikeye's Match Analysis currently spans `internal/parser/` (`single_pass.go`, `metrics.go`), `internal/storage/` (`migrations.go` at schema v1, `analysis_repo.go`), and `internal/ui/views/analysis/analysis.go` (5 tabs). See `proposal.md` for motivation and capability scope.

Key constraints observed in the codebase:
- `match_players` has `UNIQUE(demo_id, steam_id64)`, and existing queries/tests expect one row per player per demo.
- `match_rounds.player_economy_json`, `team_a_side`, `team_b_side`, and `bomb_site` are already persisted by the v1 parser, as are `match_events` (`kill`, `bomb_planted`, `bomb_defused`, `bomb_exploded`), `match_players.utility_adr`, `opening_duel_win_rate`, `awp_kills`, `one_kill_rounds`–`five_kill_rounds`, and `clutch_attempts`.
- `migrations.go` automatically snapshots existing databases to `backup/db_snapshots/` before running pending migrations.

## Goals / Non-Goals

**Goals:**
- Deliver HLTV-grade single-match analysis: `All | CT | T` side splits, `Round Swing` + `Eco-Adjusted` Rating/ADR alongside baseline `Strikeye Rating`, flash utility tracking, `1v1`–`1v5` clutch breakdowns, economy KPI cards, interactive round inspection with 2D Replay jump, and a dedicated 6th `Performance` tab (5v5 H2H matrix + weapon distribution).
- Keep all v1-parsed demos readable without errors or forced background re-parsing.

**Non-Goals:**
- Multi-demo BO3/BO5 series aggregation, HLTV map veto scraping, or external network API calls.
- Replacing the baseline `Strikeye Rating` formula (it remains intact as the default reference).

## Decisions

1. **Dedicated `match_player_sides` table vs. altering `match_players` uniqueness**
   - **Decision**: Add `match_player_sides` (`UNIQUE(demo_id, steam_id64, side)` where `side IN ('CT', 'T')`) rather than changing `match_players` to `UNIQUE(demo_id, steam_id64, side)`.
   - **Rationale**: Preserves 100% backwards compatibility for `ListMatchPlayers`, radar card generation (`internal/radar`), and existing tests that assume 10 rows per demo in `match_players`.
   - **Alternative considered**: Adding a `side_scope` column to `match_players` — rejected because every existing caller would need filtering updates.

2. **Deterministic Win-Probability Lookup Table for `Round Swing` & `Eco-Adjusted` Weighting**
   - **Decision**: Compute `Round Swing` in `internal/parser/metrics.go` using a deterministic 6x6 alive-count (`(CT_alive, T_alive)`) + `bomb_planted` win-probability table (calibrated to CS2 MR12 baselines), attributing `delta_P` to the killer (and partial credit to flash/damage assisters or clutchers). Compute `Eco-Adjusted` kill/damage weights from the ratio of victim-team vs. attacker-team round equipment value (`match_rounds.team_a_equip_value` / `team_b_equip_value`, clamped to `[0.45, 1.65]`).
   - **Rationale**: Fully deterministic, zero external ML model dependency, fast single-pass computation, and easy to unit-test in `metrics_test.go`.

3. **6-Tab Layout with Dedicated `Performance` Tab**
   - **Decision**: Insert `3. 对位与武器表现 (Performance)` between `Rounds & Economy` and `Players & CS Data Radar`, shifting Radar/2D Replay/Heatmaps to tabs 4/5/6.
   - **Rationale**: Keeps `Overview & Scoreboard` focused and scannable while giving the 5x5 H2H Kill Matrix, Opening Duel Matrix, and Weapon Distribution dedicated horizontal space.
   - **Pure-UI Aggregation for H2H & Economy KPIs**: H2H matrices, weapon distributions, economy tier win rates, and round kill feeds are derived directly in-memory from `v.rounds` and `v.events` without adding redundant summary tables.

4. **Flash Tracking via `events.PlayerFlashed` + `Kill.IsAssistedFlash`**
   - **Decision**: Hook `events.PlayerFlashed` in `single_pass.go` (ignoring `e.Attacker == nil || e.Player == nil || e.Attacker.Team == e.Player.Team` and capping/filtering trivial `<0.5s` flashes if needed or summing `e.FlashDuration.Seconds()`), while counting `FlashAssists` from `Kill.IsAssistedFlash` attributed to `e.Assister`.
   - **Rationale**: Matches HLTV's distinction between enemies blinded and flash assists without double-counting teammates.

## Risks / Trade-offs

- **Legacy v1-parsed demos lack `match_player_sides` and flash/swing columns** → Mitigation: UI detects empty `match_player_sides` / zeroed v2 fields, disables or annotates `CT`/`T` split and `Eco-Adj` with a subtle "需重新解析 Demo 以启用分阵营/闪光数据" hint, and keeps all v1 metrics + H2H matrix + Economy KPIs working (since `match_events` and `match_rounds` already exist in v1).
- **14+ columns in `TabOverview` scoreboard width** → Mitigation: Keep `PlayerName` + core columns compact (`Width(52)`–`Width(56)`), move `2K–5K`, `1v1–1v5`, `Utility ADR`, `Opening%`, `AWP`, and `Flash` into the expandable player detail drawer instead of widening the main table further.

## Migration Plan

1. Add Migration `Version: 2` (`"hltv_match_analysis_v2"`) in `internal/storage/migrations.go` using `ALTER TABLE match_players ADD COLUMN ...` and `CREATE TABLE IF NOT EXISTS match_player_sides ...`.
2. `DB.migrate` automatically snapshots existing v1 databases to `backup/db_snapshots/` prior to DDL execution.
3. Bump `ParserBaselineVersion` in `internal/version/parser_baseline.go` so newly parsed demos populate v2 columns.

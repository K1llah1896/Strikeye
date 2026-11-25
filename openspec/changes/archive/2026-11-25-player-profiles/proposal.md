# Proposal: Player Profiles

## Why

The analysis UI shows per-match player rows, and `export --steamid` can aggregate a
player across matches, but there is no persistent identity layer: users cannot
record a display-name override, scouting note, or tags ("clutch", "awper"), and
the Match Library cannot search or badge by them. The research report lists
"选手档案/标签" as part of the P1 gap (CS:DM ships player tags + tag filtering).

## What Changes

- New SQLite table `player_profiles` (migration v3) keyed by SteamID64 with
  display name, note, tags JSON, and `updated_at`.
- Repository helpers: `Upsert`, `Get`, `List`, `Delete`, `TagsForSteamIDs`,
  plus tag normalization/parsing shared by CLI and UI.
- New `internal/profile` package that combines `export.AggregatePlayer` with the
  stored profile into one report, and saves edits with merge semantics.
- `strikeye-cli profile --steamid <id64> [--name --note --tags]` prints the
  dossier or persists edits.
- Match Library cards show up to three player-tag pills, and the keyword filter
  searches tags in addition to demo/team/player text.

## Impact

- Affected specs: `player-profiles` (new)
- Affected code: `internal/storage` (migration + repo), `internal/profile` (new),
  `cmd/strikeye-cli`, `internal/ui/views/library`
- Migration is additive; existing databases upgrade in place with a pre-migration
  snapshot (existing behavior).

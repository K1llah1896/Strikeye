# Design: Player Profiles

## Storage

```sql
CREATE TABLE player_profiles (
  steam_id64   TEXT PRIMARY KEY,
  display_name TEXT NOT NULL DEFAULT '',
  note         TEXT NOT NULL DEFAULT '',
  tags_json    TEXT NOT NULL DEFAULT '[]',
  updated_at   TEXT NOT NULL
);
```

Keyed by SteamID64 because parsed demos already carry it for every player and it
survives name changes. `display_name` is an override; the parsed name stays in
`match_players` untouched. Tags are stored as a JSON array; reads normalize
(trim, drop empty, case-insensitive de-dupe) so `Star` and `star` do not both
appear.

## Merge semantics

`profile.Save` loads the existing row, replaces only the non-empty fields, and
always replaces the tag list when the caller supplies one. This lets the CLI set
a note without wiping tags, while `--tags ""` is distinguishable from omission by
passing `nil` from callers that do not edit tags.

## Aggregation

`profile.Build` wraps `export.AggregatePlayer` (summed damage/rounds, weighted
rating/KAST) so the dossier numbers match `export --steamid` exactly. No new
aggregation code path is introduced. When a display-name override exists it is
copied into `Aggregate.PlayerName` for display only.

## CLI

`strikeye-cli profile --steamid <id64>` prints
`{"aggregate": {...}, "profile": {...}, "has_profile": bool}`.
With `--name`, `--note`, or `--tags` it writes and prints
`{"saved": true, "profile": {...}}` instead. Unknown SteamIDs with no parsed
matches return a non-zero "no matches" error, matching `export --steamid`.

## UI

`library.Refresh` resolves tags per demo via `TagsForSteamIDs` (one query per
demo, demos are few) and unions them into `MatchEntry.PlayerTags`. Cards render
at most three `PillBadge`s after the favorite badge; extra tags remain searchable
but are not drawn to keep the header compact. Keyword filtering also scans tags.

## Future

Team-level profiles, tag autocomplete, and per-tag saved filters can be layered
on the same table without another migration.

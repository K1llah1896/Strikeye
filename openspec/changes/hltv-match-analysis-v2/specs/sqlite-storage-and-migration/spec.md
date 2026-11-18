# Spec Delta

## ADDED Requirements

### Requirement: Schema v2 side-split and extended match player columns
Schema migration v2 SHALL extend `match_players` with `flash_assists`, `enemies_flashed`, `total_flash_sec`, `round_swing`, `eco_adj_rating`, `eco_adj_adr`, and `clutch_breakdown_json`, and SHALL create `match_player_sides` keyed by `(demo_id, steam_id64, side)` with cascading foreign-key deletion on `demos(id)`.

#### Scenario: Migrating an existing v1 database to v2
- **WHEN** an existing v1 `strikeye.db` is opened after upgrading
- **THEN** a pre-migration snapshot is written to `backup/db_snapshots/`, all new `match_players` columns and `match_player_sides` are created, and `schema_migrations` records version `2`

### Requirement: Backwards-compatible reads for v1-parsed demos
The analysis repository SHALL read v1-parsed demo rows without error when `match_player_sides` is empty or extended `match_players` columns hold default values.

#### Scenario: Opening a legacy v1-parsed demo after migration
- **WHEN** the UI queries `ListMatchPlayers` and `ListMatchPlayerSides` for a demo parsed before migration v2
- **THEN** the queries succeed and return the stored v1 player totals with an empty side-split slice

# match-data-export

## MODIFIED Requirements

### Requirement: Per-match export
The system SHALL export a parsed match's players, rounds, events, side splits, and stored kill-context rows to CSV files and a combined JSON document, and SHALL accept `csv`, `json`, `xlsx`, `html`, `both` (csv+json) and `all` (every format) as `--format` values, including comma-separated combinations.

#### Scenario: Export both formats
- **WHEN** the operator runs `strikeye-cli export --demo-id <id> --format both --out <dir>`
- **THEN** the directory receives `match_<id>_players.csv`, `match_<id>_rounds.csv`, `match_<id>_events.csv`, `match_<id>_sides.csv`, `match_<id>_kills.csv`, and `match_<id>.json`, and their paths are printed

#### Scenario: Export every format
- **WHEN** the operator passes `--format all`
- **THEN** CSV files, `match_<id>.xlsx`, `match_<id>.html` and `match_<id>.json` are all written

#### Scenario: Export rejects unknown demo
- **WHEN** the requested demo ID does not exist
- **THEN** the command exits non-zero with an error and writes no files

#### Scenario: Export rejects unknown format
- **WHEN** the operator passes `--format pdf`
- **THEN** the command exits non-zero and names the supported values

#### Scenario: CSV headers are stable
- **WHEN** two exports of the same match are produced
- **THEN** each CSV file starts with the same header row in the same column order

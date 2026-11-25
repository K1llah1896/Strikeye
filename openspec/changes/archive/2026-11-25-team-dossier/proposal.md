# Proposal: Team Dossier

## Why

The project can inspect one match, one series, and one player, but not a team
across all parsed demos. Scouting starts with "how does this team play its
map pool and who is on the roster?" The research report lists a team-level
profile layer as part of the P1 gap; the player layer shipped first.

## What Changes

- New `internal/team` package aggregating maps, map win/loss, round totals, and
  a round-weighted roster across every parsed demo where the team name appears on
  either side.
- `strikeye-cli team --name <team>` prints the dossier as JSON.
- Roster ADR/KAST/rating are weighted by each player's rounds so a 30-round map
  does not count the same as a 16-round map.

## Impact

- Affected specs: `team-dossier` (new)
- Affected code: `internal/team` (new), `cmd/strikeye-cli`
- Read-only over existing storage; no schema change.

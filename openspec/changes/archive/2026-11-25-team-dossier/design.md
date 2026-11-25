# Design: Team Dossier

## Identity

Teams are matched by the demo's recorded `team_a_name` / `team_b_name`,
case-insensitive and trimmed. There is no persistent team entity yet: names are
free text from the demo, so the same organization spelled differently is two
teams. A team table (like `player_profiles`) can be added later without changing
this report contract.

## Side resolution

Demos store teams as `team_a`/`team_b`; when the requested name is team B, the
scoreline is flipped before aggregation so `team_score` is always from the
requested team's perspective. Players are filtered with the same side flag.

## Weighting

Each player's per-map row is multiplied by that map's round count for KAST and
rating, and their damage is summed before dividing by total rounds. This avoids
over-weighting short maps and matches the existing per-match metric definitions.

## Output

JSON only for now (CLI). `maps` is ordered by demo date (storage order), roster by
rounds then rating. The report is intentionally flat so a future UI can render it
without recomputation.

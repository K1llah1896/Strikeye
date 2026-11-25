# Spec Delta

## MODIFIED Requirements

### Requirement: Match Library search, filtering, and quick actions
The `Match Library` view SHALL query SQLite WAL for ingested demos and support filtering by keyword (map, player name, note, filename), map, platform, parse/render status, and favorites, displaying each match card anchored on prominent team names and scoreline, map/platform/status capsules, and clearly prioritized quick-navigation actions. Keyword matching SHALL be extended by full-text index hits so notes stored outside the card model stay searchable.

#### Scenario: Filter and open match analysis
- **WHEN** the user enters a map or player keyword and clicks "Match Analysis" on a parsed match card
- **THEN** the list filters to matching demos and navigates directly to `Match Analysis` for the selected demo

#### Scenario: Full-text hits extend the keyword match
- **WHEN** the keyword matches nothing in memory but the FTS index reports the demo id (for example a tactical annotation note)
- **THEN** the demo is still shown

#### Scenario: Index refresh is asynchronous
- **WHEN** the keyword changes while a previous FTS query is in flight
- **THEN** no second query is issued for that keyword until the first finishes, and the render loop never blocks on SQLite

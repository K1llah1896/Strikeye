# Spec Delta

## MODIFIED Requirements

### Requirement: Overview scoreboard and rounds/economy analysis tabs
The `Match Analysis` view SHALL provide an `Overview & Scoreboard` tab with MVP/highlight leader cards and a CT/T faction-grouped 10-player table with aligned columns sortable by K, D, A, K/D, ADR, KAST%, HS%, FK, FD, TK, Multi-Kills, Clutches, `Round Swing`, and `Strikeye Rating`, plus a `Rounds & Economy` tab plotting round-by-round win sequence badges and proportional team equipment-value bars with buy-type annotations and dynamic per-round `CT`/`T` side labels.

#### Scenario: Sort scoreboard by Strikeye Rating
- **WHEN** the user clicks the `Strikeye Rating` column header
- **THEN** player rows re-order by Strikeye Rating descending or ascending with aligned numeric columns

#### Scenario: Sort scoreboard by Round Swing
- **WHEN** the user clicks the `Round Swing` column header
- **THEN** player rows re-order by Round Swing descending or ascending with aligned numeric columns

## ADDED Requirements

### Requirement: Scoreboard side-split and eco-adjusted toggles
`Overview & Scoreboard` SHALL provide an `All | CT | T` side-split toggle and an `Eco-Adjusted` rating toggle so users can inspect side-specific performance and economy-weighted Rating/ADR without altering the baseline `Strikeye Rating` view.

#### Scenario: Filtering scoreboard by CT side
- **WHEN** the user selects `CT` in the side-split toggle on `Overview & Scoreboard`
- **THEN** the scoreboard updates all player columns to reflect only rounds played on the CT side

#### Scenario: Enabling Eco-Adjusted mode
- **WHEN** the user enables the `Eco-Adjusted` toggle
- **THEN** the scoreboard displays economy-adjusted Rating and ADR values alongside or in place of raw values with a clear mode indicator

### Requirement: Expandable player detail drawer
Clicking a player row in `Overview & Scoreboard` SHALL expand or open a player detail drawer displaying `2K/3K/4K/5K` multi-kill distribution, `1v1`–`1v5` clutch breakdown, `Utility ADR`, `Opening Duel Win%`, `AWP Kills`, and flash utility metrics (`Flash Assists`, `Enemies Flashed`, `Total Blind Sec`), degrading gracefully to zero/placeholder values for legacy v1 demos.

#### Scenario: Inspecting a player's extended breakdown
- **WHEN** the user clicks a player row in `Overview & Scoreboard`
- **THEN** the player detail drawer opens and renders multi-kill tiers, 1v1–1v5 clutch counts, utility ADR, opening duel win rate, and flash statistics

### Requirement: Economy KPI summary and interactive round inspection
`Rounds & Economy` SHALL display top-level tactical/economy KPI cards (Pistol Win%, Post-Pistol Conversion%, `Full vs Full`, `Force/Semi vs Full`, `Eco vs Full`, and A/B site Post-Plant Win%), a halftime divider in the round timeline, and clickable round rows that reveal 10-player equipment/spend details, round kill feed, and a jump-to-2D-Replay action.

#### Scenario: Inspecting a round and jumping to 2D Replay
- **WHEN** the user clicks round `R12` in `Rounds & Economy` and clicks `Jump to 2D Replay`
- **THEN** the round expands to show all 10 players' equipment values, cash spent, and kill feed, and clicking the jump button switches to the `2D Replay` tab loaded at round `12`

### Requirement: Performance tab with 5v5 H2H matrix and weapon distribution
`Match Analysis` SHALL provide a sixth tab (`Performance` / `对位与武器表现`) displaying a 5v5 Head-to-Head Kill Matrix, an Opening Duel Matrix, and a match-wide Weapon Kill Distribution computed from persisted `match_events`.

#### Scenario: Viewing 5v5 head-to-head kill matrix
- **WHEN** the user opens the `Performance` tab for a parsed match
- **THEN** the view renders a 5x5 matrix showing kill-vs-death counts between every Team A and Team B player, alongside opening duel matchups and top weapons

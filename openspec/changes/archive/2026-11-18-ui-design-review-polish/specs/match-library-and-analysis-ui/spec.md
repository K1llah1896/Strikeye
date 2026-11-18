# Spec Delta

## ADDED Requirements

### Requirement: Localized tactical labels instead of raw database enums
`Match Library` and `Match Analysis` SHALL render human-readable localized labels for parse status (`ready`, `parsing`, `pending`, `failed`), economy buy types (`full_buy`, `half_buy`, `eco`, `force_buy`, `pistol`), and round win reasons (`elimination`, `bomb_exploded`, `bomb_defused`, `time_ran_out`) rather than exposing raw snake_case identifiers.

#### Scenario: Viewing economy and match status badges
- **WHEN** a match with `ready` parse status and `full_buy` / `elimination` rounds is displayed in `Match Library` or `Rounds & Economy`
- **THEN** the UI renders localized tactical labels (e.g., `已解析`, `长枪局`, `全歼`) instead of raw database enum strings

### Requirement: Streamlined Match Library toolbar and centered scoreline anchor
The `Match Library` top toolbar SHALL avoid duplicating watch-directory management inline, and each match card SHALL anchor `Team A  Score : Score  Team B` cleanly with balanced spacing between the scoreline and match remarks.

#### Scenario: Browsing Match Library cards
- **WHEN** the user opens `Match Library`
- **THEN** the toolbar presents a compact filter/import layout and each match card displays a balanced team scoreline without a wide empty gap before the remark

### Requirement: Borderless scoreboard headers, economy column headers, and structured empty states
`Overview & Scoreboard` SHALL use a borderless, aligned table header row with active sort indicators instead of 13 bordered button boxes; `Rounds & Economy` SHALL display `TEAM A (CT)` and `TEAM B (T)` column headers; and `Radar` / `2D Replay` SHALL render structured empty-state guides (including when `replayIndex` is empty) instead of blank dark rectangles.

#### Scenario: Opening 2D Replay without cached replay frames
- **WHEN** the user opens `2D Replay` for a demo that has no `CS2RPL01` replay index
- **THEN** the round selector area displays an explicit empty-state message and the canvas shows a structured tactical placeholder guide

# Spec Delta

## ADDED Requirements

### Requirement: 2D Replay canvas displays map radar backdrop when frames are absent
When `2D Replay` is configured for a demo without `CS2RPL01` replay frames, the `560x560` radar canvas SHALL render the bundled map radar image (or procedural tactical grid fallback) instead of a solid black rectangle.

#### Scenario: Opening 2D Replay without cached replay frames
- **WHEN** the user opens `2D Replay` for a parsed `de_mirage` demo with no replay frames
- **THEN** the `560x560` canvas displays the `de_mirage` tactical radar backdrop

### Requirement: Pixel-aligned scoreboard column headers and rows
The `Overview & Scoreboard` header row and CT/T player data rows SHALL share identical horizontal padding and column widths so every metric header aligns directly above its values.

#### Scenario: Viewing Overview & Scoreboard columns
- **WHEN** the user views `Overview & Scoreboard`
- **THEN** all 13 sortable column headers (`K` through `Rating`) align vertically with the numeric cells below them

### Requirement: Collapsible manual demo import in Match Library
The `Match Library` toolbar SHALL keep manual `.dem` path entry collapsed behind a `+ 手动导入 Demo` toggle button by default, expanding the path input row on click.

#### Scenario: Toggling manual demo import
- **WHEN** the user clicks `+ 手动导入 Demo` in the `Match Library` toolbar
- **THEN** the local `.dem` path input and submit button appear below the filter bar

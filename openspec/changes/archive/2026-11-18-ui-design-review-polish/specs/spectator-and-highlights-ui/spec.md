# Spec Delta

## ADDED Requirements

### Requirement: Parsed highlight tag pills and localized clip types
The `Highlight Library` clip cards SHALL parse `TagsJSON` into clean visual tag pills (falling back gracefully if non-JSON) and render localized clip-type labels (e.g., `multi_kill` -> `多杀`, `clutch` -> `残局`) instead of raw JSON arrays or snake_case strings.

#### Scenario: Rendering a highlight clip with JSON tags
- **WHEN** a highlight clip has `ClipType: "multi_kill"` and `TagsJSON: '["4K","Headshot"]'`
- **THEN** the card displays `多杀` and separate `4K`, `Headshot` pills without raw brackets or quotes

### Requirement: Lighter Spectator round selector with overflow cue
The `Spectator` round selector SHALL use compact, low-chrome round pills inside its horizontal scroll strip and provide a subtle overflow indicator or scroll hint when rounds exceed the visible width.

#### Scenario: Viewing a 23-round match in Spectator
- **WHEN** the user opens `Spectator` for a 23-round match at `1360x840`
- **THEN** the round selector displays compact pills with a clear scroll/overflow cue instead of heavy bordered boxes clipped abruptly at the edge

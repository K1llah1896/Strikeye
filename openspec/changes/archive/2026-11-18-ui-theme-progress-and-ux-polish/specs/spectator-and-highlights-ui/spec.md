# Spec Delta

## ADDED Requirements

### Requirement: Clean highlight cards without tick range and with non-wrapping single-line tags
The `Highlight Library` clip cards SHALL omit raw tick ranges (`Ticks: xxx ~ xxx`) and SHALL render parsed tag pills inside a single-line horizontal scroll container with non-shrinking pill items so tag text never wraps across multiple lines.

#### Scenario: Viewing a highlight clip card with multiple tags
- **WHEN** a highlight clip has multiple tags (including CJK and emoji labels) in a narrow `420px` list column
- **THEN** the card does not display `Ticks: ...` and all tag pills remain on a single horizontal line scrollable without wrapping text internally

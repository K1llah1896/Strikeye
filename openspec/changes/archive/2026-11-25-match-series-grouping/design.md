# Design: Match Series Grouping

## Domain

A series is `(unordered team pair, match day)`. It is derived on demand from the
`demos` table, so re-parsing or deleting a demo immediately changes the grouping
without cache invalidation.

Orientation: teams are sorted case-insensitively to pick an anchor A/B. When a
demo's own `TeamAName` does not equal the anchor, its scores are swapped and the
map is flagged `swapped` — this keeps the series score correct for demos where
the parser assigned CT/T sides in the opposite order.

Only `parse_status = 'parsed'` demos with two real (non-placeholder) team names
participate. Groups with fewer than two maps are omitted from the series list so
standalone demos do not get a meaningless header.

## API

```go
type MapResult struct { DemoID int64; MapName string; MatchDate string; ScoreA, ScoreB int; Winner string; Swapped bool }
type Series struct { Key, TeamA, TeamB, Date string; Maps []MapResult; MapWinsA, MapWinsB int }
func Group(demos []storage.Demo) []Series
func ByDemoID(series []Series) map[int64]Series
func (s Series) Label() string // "FURIA 2-1 Falcons"
```

`Group` sorts maps by match date then demo ID, series by date descending.

## CLI

`strikeye-cli series [--data-root DIR] [--min-maps N]` prints
`{"series": [...]}`; exit 0 with an empty list when nothing qualifies.

## UI

`FilterCriteria.GroupBySeries` is a UI-only toggle. When enabled, `FilteredEntries`
keeps its order, and the render loop emits a header row before the first card of
each multi-map series (headers are skipped when the group has a single visible
card). Header uses existing `theme.MutedText` and a series pill so no new theme
tokens are required.

## Risks

- Team names come from the demo; abbreviations or sponsor churn can split a
  series. Mitigation: exact normalized match only, plus a manual `display_name`
  override is already available per demo (future: team alias table).
- Two different series between the same teams on the same day (rare in practice)
  merge into one group. Accepted for v1; the date string granularity is the
  natural key.

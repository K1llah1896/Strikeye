# Proposal: Shareable HTML Match Report

## Why

CSV/JSON exports serve pipelines, not people. Analysts share screenshots and
spreadsheets; a single self-contained HTML file opens in any browser, needs no
server, and can be attached to a review. The research report's export gap is
only half closed with machine-readable formats.

## What Changes

- `internal/export` gains an HTML writer: scoreboard per side, round timeline,
  economy-free summary, and a footer stating the schema version.
- `--format html` (and `csv,html`, `json,html`) is accepted by
  `strikeye-cli export` for both match and player exports.
- The report embeds CSS inline and escapes every dynamic string, so it is safe
  to open and contains no external requests.

## Impact

- Affected specs: `match-data-export` (added HTML requirement)
- Affected code: `internal/export` (html.go, export.go, html_test.go)
- CLI surface: new `html` value for `--format`

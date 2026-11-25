# Design: Shareable HTML Match Report

## Shape

One file per demo: `match_<id>.html`. Sections: header (teams, score, map,
date), one scoreboard table per side, a round table (round, winner, reason,
score after), and a footer with the export schema version and generator name.

## Safety

Every interpolated value goes through `html.EscapeString`. No `<link>`, `<img>`,
`<script>`, or `url(...)` references are emitted, so the file works offline and
cannot exfiltrate data. A unit test feeds a player name containing a script tag
and asserts it is escaped.

## Format selection

`ParseFormats` already splits on commas; `html` joins `csv` and `json`, and the
default remains `both` so existing pipelines are unchanged.

## Verification

`internal/export/html_test.go` checks required content, escaping, absence of
external assets, and format parsing; the CLI was run against the real two-map
fixture to produce a report that loads in a browser.

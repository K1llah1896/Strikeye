package export

import (
	"fmt"
	"html"
	"os"
	"strings"
)

// writeMatchHTML writes a self-contained, shareable match report.
func writeMatchHTML(path string, b MatchBundle) error {
	var sb strings.Builder
	demo := b.Demo
	title := fmt.Sprintf("%s vs %s", demo.TeamAName, demo.TeamBName)
	mapName := "未知地图"
	if demo.MapName != nil && *demo.MapName != "" {
		mapName = *demo.MapName
	}
	date := ""
	if demo.MatchDate != nil {
		date = *demo.MatchDate
	}
	sb.WriteString("<!doctype html>\n<html lang=\"zh\">\n<head>\n<meta charset=\"utf-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	sb.WriteString("<title>" + html.EscapeString(title) + "</title>\n")
	sb.WriteString(`<style>
:root{color-scheme:dark}
body{margin:0;padding:32px;background:#0b0f14;color:#e6edf3;font:14px/1.5 -apple-system,Segoe UI,Roboto,sans-serif}
h1{font-size:26px;margin:0 0 4px}h2{font-size:16px;margin:28px 0 10px;color:#8b98a5}
.meta{color:#8b98a5;margin-bottom:24px}
.score{font-size:22px;font-weight:600;letter-spacing:1px}
table{border-collapse:collapse;width:100%;margin-bottom:8px}
th,td{padding:6px 8px;text-align:right;border-bottom:1px solid #1e2732;white-space:nowrap}
th:first-child,td:first-child{text-align:left}
th{color:#8b98a5;font-weight:500}
tr:nth-child(even) td{background:#111821}
footer{margin-top:32px;color:#5a6672;font-size:12px}
</style>` + "\n</head>\n<body>\n")
	sb.WriteString("<h1>" + html.EscapeString(title) + "</h1>\n")
	sb.WriteString("<div class=\"meta\">" + html.EscapeString(mapName) + " · " + html.EscapeString(date) + "</div>\n")
	sb.WriteString(fmt.Sprintf("<div class=\"score\">%d : %d</div>\n", demo.TeamAScore, demo.TeamBScore))

	for _, team := range []struct {
		name string
		key  string
	}{{demo.TeamAName, "team_a"}, {demo.TeamBName, "team_b"}} {
		sb.WriteString("<h2>" + html.EscapeString(team.name) + "</h2>\n")
		sb.WriteString("<table><thead><tr><th>选手</th><th>K</th><th>D</th><th>A</th><th>ADR</th><th>KAST</th><th>HS%</th><th>Rating</th></tr></thead><tbody>\n")
		for _, p := range b.Players {
			if p.TeamKey != team.key {
				continue
			}
			sb.WriteString("<tr>")
			sb.WriteString("<td>" + html.EscapeString(p.PlayerName) + "</td>")
			sb.WriteString(fmt.Sprintf("<td>%d</td><td>%d</td><td>%d</td><td>%.1f</td><td>%.0f%%</td><td>%.0f%%</td><td>%.2f</td>",
				p.Kills, p.Deaths, p.Assists, p.ADR, p.KASTPercent, p.HSPercent, p.StrikeyeRating))
			sb.WriteString("</tr>\n")
		}
		sb.WriteString("</tbody></table>\n")
	}

	sb.WriteString("<h2>回合</h2>\n<table><thead><tr><th>#</th><th>胜方</th><th>方式</th><th>比分</th></tr></thead><tbody>\n")
	for _, r := range b.Rounds {
		winner := "-"
		if r.WinnerSide != nil && *r.WinnerSide != "" {
			winner = *r.WinnerSide
		} else if r.WinnerTeamKey != nil {
			winner = *r.WinnerTeamKey
		}
		reason := ""
		if r.WinReason != nil {
			reason = *r.WinReason
		}
		score := fmt.Sprintf("%d-%d", r.TeamAScoreAfter, r.TeamBScoreAfter)
		sb.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			r.RoundNumber, html.EscapeString(winner), html.EscapeString(reason), html.EscapeString(score)))
	}
	sb.WriteString("</tbody></table>\n")
	sb.WriteString(fmt.Sprintf("<footer>Strikeye · schema v%d · demo %d · %d rounds<br>%s</footer>\n",
		SchemaVersion, demo.ID, demo.TotalRounds, html.EscapeString(demo.FilePath)))
	sb.WriteString("</body>\n</html>\n")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write html report: %w", err)
	}
	return nil
}

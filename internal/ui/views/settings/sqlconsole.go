package settings

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/sqlconsole"
	"strikeye/internal/ui/theme"
)

// sqlConsoleLimit caps the rows a UI query returns; the console is for spot
// checks, not for dumping the database.
const sqlConsoleLimit = 200

// sqlPresets are the queries analysts actually type, so nobody has to remember
// the schema from the console.
var sqlPresets = []string{
	"select demo_id, player_name, adr, kast_percent, strikeye_rating from match_players order by strikeye_rating desc limit 20",
	"select map_name, count(*) as demos from demos group by map_name order by demos desc",
	"select round_number, team_a_side, winner_side, win_reason from match_rounds where demo_id = 1 order by round_number",
	"select weapon, count(*) as kills from match_events where event_type = 'kill' group by weapon order by kills desc limit 20",
}

// formatCell renders any driver value for display and CSV.
func formatCell(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(val, 10)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// writeQueryCSV stores a console result under <DataRoot>/media/exports.
func writeQueryCSV(result sqlconsole.Result, dataRoot string, now time.Time) (string, error) {
	dir := filepath.Join(dataRoot, "media", "exports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("query_%s.csv", now.Format("20060102_150405")))
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	w := csv.NewWriter(file)
	if err := w.Write(result.Columns); err != nil {
		return "", err
	}
	for _, row := range result.Rows {
		record := make([]string, len(row))
		for i := range row {
			record[i] = formatCell(row[i])
		}
		if err := w.Write(record); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return path, nil
}

// runConsoleQuery validates and executes a read-only query off the UI thread.
func (v *View) runConsoleQuery() {
	query := strings.TrimSpace(v.sqlQuery)
	if query == "" {
		return
	}
	if err := sqlconsole.Validate(query); err != nil {
		v.updateMu.Lock()
		v.sqlErr = err.Error()
		v.sqlBusy = false
		v.updateMu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := v.db.ReadOnly(ctx)
	if err != nil {
		v.updateMu.Lock()
		v.sqlErr = err.Error()
		v.sqlBusy = false
		v.updateMu.Unlock()
		return
	}
	defer conn.Close()
	result, err := sqlconsole.Run(ctx, conn, query, sqlConsoleLimit)
	v.updateMu.Lock()
	v.sqlBusy = false
	if err != nil {
		v.sqlErr = err.Error()
	} else {
		v.sqlErr = ""
		v.sqlResult = result
	}
	v.updateMu.Unlock()
	v.invalidate()
}

// renderSQLConsole draws the read-only query card.
func (v *View) renderSQLConsole(c *ui.Context) {
	theme.BentoCard(c, ui.Color{}, 12, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.RenderIcon(c, theme.IconAnalysis, 15, theme.AccentCyan)
			theme.SectionTitle(c, "只读 SQL 控制台")
			ui.Spacer(c)
			ui.Select(c, &v.sqlPreset, sqlPresets).Width(360)
			if theme.ActionButton(c, "载入").Clicked() {
				v.sqlQuery = v.sqlPreset
			}
		})
		theme.MutedText(c, "只允许 select / with / explain；连接以 query_only 打开，写语句会被校验层与 SQLite 双重拒绝。")
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &v.sqlQuery).FillWidth().Placeholder("select …")
			if theme.AccentButton(c, "运行").Clicked() && !v.sqlBusy && v.db != nil {
				v.sqlBusy = true
				v.sqlErr = ""
				go v.runConsoleQuery()
			}
			if v.sqlBusy {
				theme.PillBadge(c, "查询中...", theme.AccentAmber)
			}
		})
		if v.sqlErr != "" {
			ui.Text(c, v.sqlErr).FontSize(12).TextColor(theme.StatusRed)
		}
		result := v.sqlResult
		if len(result.Columns) == 0 {
			return
		}
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.MutedText(c, fmt.Sprintf("%d 行 · %d 列", len(result.Rows), len(result.Columns)))
			if result.Truncated {
				theme.PillBadge(c, fmt.Sprintf("已截断到 %d 行", sqlConsoleLimit), theme.AccentAmber)
			}
			ui.Spacer(c)
			if theme.ActionButton(c, "导出 CSV").Clicked() {
				path, err := writeQueryCSV(result, v.dataRoot, time.Now())
				if err != nil {
					v.setStatus("导出失败: " + err.Error())
				} else {
					v.setStatus("已导出 " + path)
				}
			}
		})
		header := strings.Join(result.Columns, "  |  ")
		ui.Text(c, header).Font("Consolas").FontSize(11).TextColor(theme.AccentCyan)
		for i, row := range result.Rows {
			if i >= 20 {
				theme.MutedText(c, fmt.Sprintf("… 其余 %d 行请导出 CSV", len(result.Rows)-20))
				break
			}
			cells := make([]string, len(row))
			for j := range row {
				cells[j] = formatCell(row[j])
			}
			ui.Text(c, strings.Join(cells, "  |  ")).Font("Consolas").FontSize(11).TextColor(theme.TextPrimary)
		}
	})
}

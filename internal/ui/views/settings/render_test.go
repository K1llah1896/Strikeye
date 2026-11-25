package settings

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/storage"
	"strikeye/internal/ui/renderutil"
)

// TestRenderAllCards guards every settings card against nil-pointer and layout
// panics: GSI setup, map data, SQL console and the rest all render in one pass.
func TestRenderAllCards(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	dataRoot := t.TempDir()
	v := NewView(db, nil, dataRoot)
	if got := renderutil.AssertRendered(t, 50, func(c *ui.Context) { v.Render(c) }); got < 50 {
		t.Fatalf("colours = %d", got)
	}
}

// TestRenderSQLConsoleWithResult renders the console with a populated result so
// the table branch is exercised.
func TestRenderSQLConsoleWithResult(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	v := NewView(db, nil, t.TempDir())
	v.sqlQuery = "select 1"
	v.sqlResult.Columns = []string{"map_name", "rounds"}
	v.sqlResult.Rows = [][]any{{"de_dust2", int64(19)}, {"de_ancient", int64(30)}}
	v.sqlErr = "示例错误"
	if got := renderutil.AssertRendered(t, 20, func(c *ui.Context) { v.Render(c) }); got < 20 {
		t.Fatalf("colours = %d", got)
	}
}

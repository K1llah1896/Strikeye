package sqlconsole

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"strikeye/internal/storage"
)

func testDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "strikeye.db"), "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func sqlDB(t *testing.T, db *storage.DB) *sql.DB {
	t.Helper()
	conn, err := db.ReadOnly(context.Background())
	if err != nil {
		t.Fatalf("read-only: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestValidateAllowsReadQueries(t *testing.T) {
	ok := []string{
		"SELECT 1",
		"select name from demos;",
		"WITH x AS (SELECT 1 AS n) SELECT n FROM x",
		"EXPLAIN SELECT 1",
		"SELECT 'delete from demos' AS text",
		"SELECT count(*) FROM match_events WHERE event_type = 'kill'",
	}
	for _, q := range ok {
		if err := Validate(q); err != nil {
			t.Fatalf("%q rejected: %v", q, err)
		}
	}
}

func TestValidateRejectsWrites(t *testing.T) {
	bad := []string{
		"",
		"   ",
		"DELETE FROM demos",
		"insert into demos(id) values (1)",
		"UPDATE demos SET map_name='x'",
		"DROP TABLE demos",
		"ATTACH DATABASE '/tmp/x.db' AS x",
		"PRAGMA journal_mode=DELETE",
		"SELECT 1; DELETE FROM demos",
		"CREATE TABLE x(a)",
	}
	for _, q := range bad {
		if err := Validate(q); err == nil {
			t.Fatalf("%q accepted", q)
		}
	}
}

func TestRunReturnsColumnsAndCapsRows(t *testing.T) {
	db := testDB(t)
	conn := sqlDB(t, db)
	res, err := Run(context.Background(), conn, "SELECT 1 AS a, 'x' AS b UNION ALL SELECT 2, 'y'", 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Columns) != 2 || res.Columns[0] != "a" || res.Columns[1] != "b" {
		t.Fatalf("columns = %v", res.Columns)
	}
	if len(res.Rows) != 1 || !res.Truncated {
		t.Fatalf("rows = %v truncated=%v", res.Rows, res.Truncated)
	}
	if got, ok := res.Rows[0][0].(int64); !ok || got != 1 {
		t.Fatalf("cell = %#v", res.Rows[0][0])
	}
}

func TestRunAcceptsWithAndSemicolon(t *testing.T) {
	db := testDB(t)
	conn := sqlDB(t, db)
	res, err := Run(context.Background(), conn, "WITH x AS (SELECT 'ok' AS v) SELECT v FROM x;", 10)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "ok" {
		t.Fatalf("rows = %v", res.Rows)
	}
	if res.Truncated {
		t.Fatal("unexpected truncation")
	}
}

func TestRunReportsBadSQL(t *testing.T) {
	db := testDB(t)
	conn := sqlDB(t, db)
	if _, err := Run(context.Background(), conn, "SELECT * FROM no_such_table", 10); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("err = %v", err)
	}
}

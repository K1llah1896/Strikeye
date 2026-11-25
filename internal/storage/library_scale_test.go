package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// seedLargeLibrary inserts n demos with players so pagination and search can be
// exercised against a realistic library size.
func seedLargeLibrary(t testing.TB, db *DB, n int) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.SQL().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339)
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO demos (file_path, filename, content_sha256, file_size, map_name, team_a_name, team_b_name,
			team_a_score, team_b_score, tick_rate, parse_status, added_at, is_favorite)
		VALUES (?, ?, ?, 1024, ?, ?, ?, ?, ?, 64, 'parsed', ?, 0)`)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	maps := []string{"de_dust2", "de_mirage", "de_nuke", "de_ancient"}
	for i := 0; i < n; i++ {
		if _, err := stmt.ExecContext(ctx,
			fmt.Sprintf("/demos/d%d.dem", i), fmt.Sprintf("d%d.dem", i), fmt.Sprintf("sha%d", i),
			maps[i%len(maps)], fmt.Sprintf("Team%d", i%50), fmt.Sprintf("Rival%d", i%37),
			i%16, (i*7)%16, now,
		); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestLargeLibraryPaginationAndSearch(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	const total = 2000
	seedLargeLibrary(t, db, total)

	count, err := db.Demos().CountDemos(ctx)
	if err != nil || count != total {
		t.Fatalf("count = %d, %v", count, err)
	}

	start := time.Now()
	page, err := db.Demos().ListPage(ctx, 50, 950)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	pageCost := time.Since(start)
	if len(page) != 50 {
		t.Fatalf("page size = %d", len(page))
	}
	// Offset paging must stay well under a frame budget on a 2k library.
	if pageCost > 200*time.Millisecond {
		t.Fatalf("paging took %s", pageCost)
	}

	start = time.Now()
	rebuilt, err := db.Search().RebuildSearchIndex(ctx)
	indexCost := time.Since(start)
	if err != nil || rebuilt != total {
		t.Fatalf("rebuild = %d, %v", rebuilt, err)
	}
	start = time.Now()
	hits, err := db.Search().Query(ctx, "dust2", 50)
	queryCost := time.Since(start)
	if err != nil || len(hits) == 0 {
		t.Fatalf("query hits = %d, %v", len(hits), err)
	}
	if queryCost > 100*time.Millisecond {
		t.Fatalf("FTS query took %s", queryCost)
	}
	t.Logf("2000 demos: page(50,950)=%s index=%s fts=%s hits=%d", pageCost, indexCost, queryCost, len(hits))
}

func BenchmarkDemoListPage(b *testing.B) {
	db, err := OpenDefault(b.TempDir())
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer db.Close()
	seedLargeLibrary(b, db, 2000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := db.Demos().ListPage(ctx, 50, i*50%1000); err != nil {
			b.Fatalf("page: %v", err)
		}
	}
}

func BenchmarkSearchQuery(b *testing.B) {
	db, err := OpenDefault(b.TempDir())
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer db.Close()
	seedLargeLibrary(b, db, 2000)
	ctx := context.Background()
	if _, err := db.Search().RebuildSearchIndex(ctx); err != nil {
		b.Fatalf("rebuild: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := db.Search().Query(ctx, "dust2", 20); err != nil {
			b.Fatalf("query: %v", err)
		}
	}
}

// Package storage provides SQLite WAL persistence, versioned schema migrations,
// repository DAOs, and lossless legacy data import for Strikeye.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a configured SQLite handle with WAL mode, foreign keys, and migrations.
type DB struct {
	sql         *sql.DB
	path        string
	snapshotDir string
}

// Open opens (or creates) the SQLite database at dbPath, applies WAL and foreign-key
// pragmas, takes a pre-migration snapshot when upgrading an existing database, and
// runs all pending migrations.
func Open(dbPath, snapshotDir string, migrations ...Migration) (*DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("dbPath cannot be empty")
	}
	cleanPath := filepath.Clean(dbPath)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db parent dir: %w", err)
	}
	if snapshotDir != "" {
		if err := os.MkdirAll(filepath.Clean(snapshotDir), 0o755); err != nil {
			return nil, fmt.Errorf("create snapshot dir: %w", err)
		}
	}

	if len(migrations) == 0 {
		migrations = DefaultMigrations()
	}

	dsn := buildSQLiteDSN(cleanPath)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", cleanPath, err)
	}
	// Single writer connection guarantees PRAGMAs apply consistently and avoids SQLITE_BUSY in-process.
	sqlDB.SetMaxOpenConns(1)

	db := &DB{
		sql:         sqlDB,
		path:        cleanPath,
		snapshotDir: filepath.Clean(snapshotDir),
	}

	if err := db.applyPragmas(context.Background()); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := db.migrate(context.Background(), migrations); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// OpenDefault opens <dataRoot>/strikeye.db with snapshots in <dataRoot>/backup/db_snapshots.
func OpenDefault(dataRoot string) (*DB, error) {
	if dataRoot == "" {
		return nil, fmt.Errorf("dataRoot cannot be empty")
	}
	clean := filepath.Clean(dataRoot)
	return Open(
		filepath.Join(clean, "strikeye.db"),
		filepath.Join(clean, "backup", "db_snapshots"),
	)
}

func isUNCPath(path string) bool {
	slashed := strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
	return strings.HasPrefix(slashed, "//")
}

func buildSQLiteDSN(path string) string {
	slashed := strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
	// Windows UNC paths (\\host\share\..., e.g. \\wsl.localhost\...) do not
	// support SQLite WAL shared-memory locking reliably; use DELETE journal mode
	// and empty URI authority (file:////host/share/...).
	if strings.HasPrefix(slashed, "//") {
		return "file://" + slashed + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(DELETE)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&vfs=win32-none"
	}
	return "file:" + slashed + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
}

func (db *DB) applyPragmas(ctx context.Context) error {
	basePragmas := []string{
		"PRAGMA busy_timeout=5000;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range basePragmas {
		if _, err := db.sql.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("exec %q: %w", p, err)
		}
	}
	if isUNCPath(db.path) {
		if _, err := db.sql.ExecContext(ctx, "PRAGMA journal_mode=DELETE;"); err != nil {
			return fmt.Errorf("exec %q: %w", "PRAGMA journal_mode=DELETE;", err)
		}
		return nil
	}
	if _, err := db.sql.ExecContext(ctx, "PRAGMA journal_mode=WAL;"); err != nil {
		// If another process is concurrently initializing the DB, fall back to DELETE
		// instead of failing desktop startup with SQLITE_BUSY.
		if _, fbErr := db.sql.ExecContext(ctx, "PRAGMA journal_mode=DELETE;"); fbErr != nil {
			return fmt.Errorf("exec %q: %w", "PRAGMA journal_mode=WAL;", err)
		}
	}
	return nil
}

// Close closes the underlying SQLite connection.
func (db *DB) Close() error {
	if db == nil || db.sql == nil {
		return nil
	}
	return db.sql.Close()
}

// SQL exposes the underlying *sql.DB for advanced callers or tests.
func (db *DB) SQL() *sql.DB {
	return db.sql
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// Snapshot writes a consistent standalone SQLite copy of the database to destPath
// using VACUUM INTO.
func (db *DB) Snapshot(ctx context.Context, destPath string) error {
	if destPath == "" {
		return fmt.Errorf("snapshot destination cannot be empty")
	}
	cleanDest := filepath.Clean(destPath)
	if err := os.MkdirAll(filepath.Dir(cleanDest), 0o755); err != nil {
		return fmt.Errorf("create snapshot parent dir: %w", err)
	}
	escaped := strings.ReplaceAll(filepath.ToSlash(cleanDest), "'", "''")
	if _, err := db.sql.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		return fmt.Errorf("vacuum snapshot into %q: %w", cleanDest, err)
	}
	return nil
}

func utcNowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

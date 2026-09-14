package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open creates a SQLite connection, applies embedded migrations, and verifies
// that the database is reachable. A single connection avoids SQLite in-memory
// database surprises and keeps writes serialized for this local application.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		directory := filepath.Dir(path)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configure database: %w", err)
		}
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// Migrate applies each embedded numbered migration exactly once.
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := rebuildIncompatibleLegacySchema(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version TEXT PRIMARY KEY,
        applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	for _, name := range names {
		var applied int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied != 0 {
			continue
		}
		contents, err := migrationFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// rebuildIncompatibleLegacySchema handles the pre-migration hackathon schema.
// Those databases contained only reproducible seed data, so rebuilding the two
// incompatible tables is safer than attempting to reinterpret their columns.
func rebuildIncompatibleLegacySchema(ctx context.Context, db *sql.DB) error {
	legacy, err := tableMissingColumn(ctx, db, "broadcasts", "source")
	if err != nil {
		return err
	}
	legacySenders, err := tableMissingColumn(ctx, db, "senders", "profile_source")
	if err != nil {
		return err
	}
	if !legacy && !legacySenders {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin legacy schema rebuild: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{
		"DROP TABLE IF EXISTS broadcasts",
		"DROP TABLE IF EXISTS senders",
		"DROP TABLE IF EXISTS schema_migrations",
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("rebuild legacy schema: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy schema rebuild: %w", err)
	}
	return nil
}

func tableMissingColumn(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
        WHERE type = 'table' AND name = ?`, table).Scan(&exists); err != nil {
		return false, fmt.Errorf("inspect table %s: %w", table, err)
	}
	if exists == 0 {
		return false, nil
	}
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, fmt.Errorf("inspect columns for %s: %w", table, err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan columns for %s: %w", table, err)
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate columns for %s: %w", table, err)
	}
	return !found, nil
}

// IsDatasetEmpty reports whether the database needs its initial archive import.
func IsDatasetEmpty(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM broadcasts WHERE source = 'dataset'").Scan(&count); err != nil {
		return false, fmt.Errorf("count imported broadcasts: %w", err)
	}
	return count == 0, nil
}

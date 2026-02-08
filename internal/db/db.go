package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amxv/adm/internal/pathnorm"
	_ "modernc.org/sqlite"
)

// Open finds the project root, ensures the .agents/adm/ directory exists,
// opens the SQLite database, sets pragmas, and runs migrations.
func Open() (*sql.DB, error) {
	root, err := pathnorm.FindRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("find project root: %w", err)
	}

	dir := filepath.Join(root, ".agents", "adm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dir, "adm.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := setPragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func setPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

const currentSchemaVersion = 3

func migrate(d *sql.DB) error {
	// Fast path: skip DDL if schema is already at the current version.
	var version int
	if err := d.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version >= currentSchemaVersion {
		return nil
	}

	// First-time or upgrade: run full migrations.
	if _, err := d.Exec(schemaV1); err != nil {
		return fmt.Errorf("schema v1: %w", err)
	}
	if version < 2 {
		if _, err := d.Exec(migrateV2); err != nil {
			return fmt.Errorf("schema v2: %w", err)
		}
	}
	if version < 3 {
		if _, err := d.Exec(migrateV3); err != nil {
			return fmt.Errorf("schema v3: %w", err)
		}
	}
	if _, err := d.Exec(fmt.Sprintf("PRAGMA user_version = %d", currentSchemaVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return nil
}

package store

import (
	"context"
	"embed"
	"io/fs"
	"sort"

	"database/sql"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations executes SQL files in alphabetical order within the migrations folder.
// Each file is executed in a single transaction.
func RunMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	// ensure deterministic order: 001_..., 002_..., etc.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sqlBytes, err := fs.ReadFile(migrationsFS, "migrations/"+e.Name())
		if err != nil {
			return err
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

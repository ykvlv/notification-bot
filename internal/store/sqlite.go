package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Registers the "sqlite" driver (pure Go).
	_ "modernc.org/sqlite"

	"github.com/ykvlv/notification-bot/internal/domain"
)

// SQLiteRepo implements Repo using an embedded SQLite database.
type SQLiteRepo struct{ db *sql.DB }

// OpenSQLite opens (or creates) the SQLite database at the given path,
// applies recommended PRAGMAs, runs SQL migrations, and returns a repository.
func OpenSQLite(ctx context.Context, path string) (*SQLiteRepo, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Reasonable pooling for SQLite; it's a single-writer engine.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Apply PRAGMAs and run migrations.
	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply pragmas: %w", err)
	}
	if err := RunMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return &SQLiteRepo{db: db}, nil
}

// applyPragmas configures the SQLite connection for durability and concurrency.
func applyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the underlying database resources.
func (r *SQLiteRepo) Close() error {
	return r.db.Close()
}

// UpsertUser inserts or updates a user's settings and schedule.
// If the user (chat_id) exists, fields are updated; otherwise, a new row is inserted.
func (r *SQLiteRepo) UpsertUser(ctx context.Context, u *domain.User) error {
	if u == nil {
		return errors.New("nil user")
	}

	now := time.Now().UTC().Unix()
	created := u.CreatedAt.UTC().Unix()
	if created == 0 {
		created = now
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (
			chat_id, created_at, enabled, tz, interval_sec,
			active_from_m, active_to_m, message, next_fire_at, last_sent_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET
			enabled       = excluded.enabled,
			tz            = excluded.tz,
			interval_sec  = excluded.interval_sec,
			active_from_m = excluded.active_from_m,
			active_to_m   = excluded.active_to_m,
			message       = excluded.message,
			next_fire_at  = excluded.next_fire_at,
			last_sent_at  = excluded.last_sent_at`,
		u.ChatID, created, boolToInt(u.Enabled), u.TZ, u.IntervalSec,
		u.ActiveFromM, u.ActiveToM, u.Message,
		toNullInt64(u.NextFireAt), toNullInt64(u.LastSentAt),
	)
	return err
}

// GetUser returns a user's settings by chatID or an error if not found.
func (r *SQLiteRepo) GetUser(ctx context.Context, chatID int64) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT chat_id, created_at, enabled, tz, interval_sec,
		       active_from_m, active_to_m, message,
		       next_fire_at, last_sent_at
		FROM users
		WHERE chat_id = ?`,
		chatID,
	)

	var (
		chatIDOut   int64
		createdAt   int64
		enabledInt  int
		tz          string
		intervalSec int
		activeFromM int
		activeToM   int
		message     string
		nextNS      sql.NullInt64
		lastNS      sql.NullInt64
	)

	if err := row.Scan(
		&chatIDOut, &createdAt, &enabledInt, &tz, &intervalSec,
		&activeFromM, &activeToM, &message, &nextNS, &lastNS,
	); err != nil {
		return nil, err
	}

	return &domain.User{
		ChatID:      chatIDOut,
		Enabled:     enabledInt != 0,
		TZ:          tz,
		IntervalSec: intervalSec,
		ActiveFromM: activeFromM,
		ActiveToM:   activeToM,
		Message:     message,
		NextFireAt:  fromNullInt64(nextNS),
		LastSentAt:  fromNullInt64(lastNS),
		CreatedAt:   time.Unix(createdAt, 0).UTC(),
	}, nil
}

// ListDue returns up to `limit` users whose next_fire_at is <= now and are enabled.
// Results are ordered by next_fire_at ascending.
func (r *SQLiteRepo) ListDue(ctx context.Context, now time.Time, limit int) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT chat_id, created_at, enabled, tz, interval_sec,
		       active_from_m, active_to_m, message, next_fire_at, last_sent_at
		FROM users
		WHERE enabled = 1
		  AND next_fire_at IS NOT NULL
		  AND next_fire_at <= ?
		ORDER BY next_fire_at ASC
		LIMIT ?`,
		now.UTC().Unix(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.User
	for rows.Next() {
		var (
			chatIDOut   int64
			createdAt   int64
			enabledInt  int
			tz          string
			intervalSec int
			activeFromM int
			activeToM   int
			message     string
			nextNS      sql.NullInt64
			lastNS      sql.NullInt64
		)

		if err := rows.Scan(
			&chatIDOut, &createdAt, &enabledInt, &tz, &intervalSec,
			&activeFromM, &activeToM, &message, &nextNS, &lastNS,
		); err != nil {
			return nil, err
		}

		res = append(res, domain.User{
			ChatID:      chatIDOut,
			Enabled:     enabledInt != 0,
			TZ:          tz,
			IntervalSec: intervalSec,
			ActiveFromM: activeFromM,
			ActiveToM:   activeToM,
			Message:     message,
			NextFireAt:  fromNullInt64(nextNS),
			LastSentAt:  fromNullInt64(lastNS),
			CreatedAt:   time.Unix(createdAt, 0).UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// SetSchedule updates next_fire_at and (optionally) last_sent_at for a user.
func (r *SQLiteRepo) SetSchedule(ctx context.Context, chatID int64, next time.Time, last *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET next_fire_at = ?, last_sent_at = ?
		WHERE chat_id = ?`,
		next.UTC().Unix(), toNullInt64(last), chatID,
	)
	return err
}

// SetEnabled toggles the enabled flag for a user.
func (r *SQLiteRepo) SetEnabled(ctx context.Context, chatID int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET enabled = ?
		WHERE chat_id = ?`,
		boolToInt(enabled), chatID,
	)
	return err
}

// boolToInt converts a boolean to 1/0 for SQLite.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

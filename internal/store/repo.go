package store

import (
	"context"
	"time"

	"github.com/ykvlv/notification-bot/internal/domain"
)

// Repo defines storage operations for users and scheduling.
type Repo interface {
	UpsertUser(ctx context.Context, u *domain.User) error
	GetUser(ctx context.Context, chatID int64) (*domain.User, error)
	ListDue(ctx context.Context, now time.Time, limit int) ([]domain.User, error)
	SetSchedule(ctx context.Context, chatID int64, next time.Time, last *time.Time) error
	SetEnabled(ctx context.Context, chatID int64, enabled bool) error
	Close() error
}

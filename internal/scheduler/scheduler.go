package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/ykvlv/notification-bot/internal/domain"
	"github.com/ykvlv/notification-bot/internal/store"
)

// Sender is a minimal interface the scheduler needs to send a text message.
// telegram.Router will implement this (method: SendMessage).
type Sender interface {
	SendMessage(chatID int64, text string) error
}

// Scheduler periodically polls the DB and dispatches due notifications.
type Scheduler struct {
	repo     store.Repo
	log      *zap.Logger
	sender   Sender
	interval time.Duration
}

// New creates a new Scheduler. Poll interval is fixed for MVP (30s).
func New(repo store.Repo, log *zap.Logger, sender Sender) *Scheduler {
	return &Scheduler{
		repo:     repo,
		log:      log,
		sender:   sender,
		interval: 30 * time.Second,
	}
}

// Run starts the loop until ctx is canceled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopping")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick performs one scheduling cycle: find due users, send, reschedule.
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().UTC()

	users, err := s.repo.ListDue(ctx, now, 100)
	if err != nil {
		s.log.Error("ListDue failed", zap.Error(err))
		return
	}
	for _, u := range users {
		// Send user's message
		if err := s.sender.SendMessage(u.ChatID, u.Message); err != nil {
			s.log.Error("send failed", zap.Error(err), zap.Int64("chatID", u.ChatID))
			continue
		}

		// Compute next fire time and persist
		next := domain.NextFire(now, &u)
		if err := s.repo.SetSchedule(ctx, u.ChatID, next, &now); err != nil {
			s.log.Error("SetSchedule failed", zap.Error(err), zap.Int64("chatID", u.ChatID))
		}
	}
}

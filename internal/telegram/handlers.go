package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/ykvlv/notification-bot/internal/domain"
)

const (
	defaultTZ       = "Europe/Moscow"
	defaultInterval = 2 * time.Hour
	defaultFromM    = 9 * 60  // 09:00
	defaultToM      = 22 * 60 // 22:00
	defaultMessage  = "Achtung 🚨"
)

// ensureUser makes sure a user row exists; if not, creates with defaults.
func (r *Router) ensureUser(ctx context.Context, chatID int64) (*domain.User, error) {
	u, err := r.repo.GetUser(ctx, chatID)
	if err == nil {
		return u, nil
	}
	// if not found, create defaults
	now := time.Now().UTC()
	u = &domain.User{
		ChatID:      chatID,
		Enabled:     true,
		TZ:          defaultTZ,
		IntervalSec: int(defaultInterval.Seconds()),
		ActiveFromM: defaultFromM,
		ActiveToM:   defaultToM,
		Message:     defaultMessage,
		NextFireAt:  nil,
		LastSentAt:  nil,
		CreatedAt:   now,
	}
	if err := r.repo.UpsertUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Router) handleStart(ctx context.Context, chatID int64) {
	if _, err := r.ensureUser(ctx, chatID); err != nil {
		r.log.Error("ensureUser failed", zap.Error(err))
		r.sendText(chatID, "Profile initialization error. Please try again later.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, startText)
	kb := mainMenuKeyboard()
	msg.ReplyMarkup = kb
	r.bot.Send(msg)
}

func (r *Router) handleStatus(ctx context.Context, chatID int64) {
	u, err := r.ensureUser(ctx, chatID)
	if err != nil {
		r.log.Error("ensureUser failed", zap.Error(err))
		r.sendText(chatID, "Error reading your settings.")
		return
	}

	interval := time.Duration(u.IntervalSec) * time.Second
	activeFrom := domain.FormatMinutes(u.ActiveFromM)
	activeTo := domain.FormatMinutes(u.ActiveToM)
	enabled := "✅ Enabled"
	if !u.Enabled {
		enabled = "⏸ Paused"
	}
	next := "—"
	if u.NextFireAt != nil {
		if s, err := domain.LocalizeTime(*u.NextFireAt, u.TZ); err == nil {
			next = s
		}
	}

	body := fmt.Sprintf("%s\n\n"+statusFmt,
		statusTitle,
		interval.String(),
		activeFrom, activeTo,
		u.TZ,
		enabled,
		next,
	)
	msg := tgbotapi.NewMessage(chatID, body)
	msg.ReplyMarkup = mainMenuKeyboard()
	r.bot.Send(msg)
}

func (r *Router) handleSettings(ctx context.Context, chatID int64) {
	if _, err := r.ensureUser(ctx, chatID); err != nil {
		r.log.Error("ensureUser failed", zap.Error(err))
		r.sendText(chatID, "Error opening settings.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "What do you want to configure?")
	kb := settingsInlineKeyboard()
	msg.ReplyMarkup = kb
	r.bot.Send(msg)
}

// ===== Interval flow =====

func (r *Router) askIntervalPresets(ctx context.Context, chatID int64, cbID string) {
	_ = r.answerCallback(cbID, "")
	msg := tgbotapi.NewMessage(chatID, "Choose an interval (or Custom to enter your own):")
	msg.ReplyMarkup = intervalPresetsKeyboard()
	r.bot.Send(msg)
}

func (r *Router) handleIntervalCallback(ctx context.Context, chatID int64, data string, cbID string) {
	_ = r.answerCallback(cbID, "")
	if data == "interval:custom" {
		r.sendText(chatID, "Enter interval, e.g.: 30m, 1h, 1h30m, 90m")
		// Simple state approach (MVP): expect next free-form message to be duration
		r.setPending(chatID, "await_interval_text")
		return
	}
	// data like "interval:2h"
	val := strings.TrimPrefix(data, "interval:")
	dur, err := domain.ParseDurationHuman(val)
	if err != nil {
		r.sendText(chatID, "Invalid interval. Examples: 30m, 1h, 1h30m.")
		return
	}
	if err := r.updateInterval(ctx, chatID, dur); err != nil {
		r.log.Error("updateInterval failed", zap.Error(err))
		r.sendText(chatID, "Could not save interval.")
		return
	}
	r.sendText(chatID, "Interval updated: "+dur.String())
}

func (r *Router) handleFreeForm(ctx context.Context, chatID int64, text string) {
	switch r.getPending(chatID) {
	case "await_interval_text":
		r.clearPending(chatID)
		dur, err := domain.ParseDurationHuman(text)
		if err != nil {
			r.sendText(chatID, "Invalid interval. Examples: 30m, 1h, 1h30m.")
			return
		}
		if err := r.updateInterval(ctx, chatID, dur); err != nil {
			r.log.Error("updateInterval failed", zap.Error(err))
			r.sendText(chatID, "Could not save interval.")
			return
		}
		r.sendText(chatID, "Interval updated: "+dur.String())
	default:
		// ignore or future logic for message / tz / hours
	}
}

func (r *Router) updateInterval(ctx context.Context, chatID int64, d time.Duration) error {
	u, err := r.ensureUser(ctx, chatID)
	if err != nil {
		return err
	}
	u.IntervalSec = int(d.Seconds())
	return r.repo.UpsertUser(ctx, u)
}

// ===== small helpers =====

func (r *Router) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	r.bot.Send(msg)
}

func (r *Router) answerCallback(id, text string) error {
	_, err := r.bot.Request(tgbotapi.NewCallback(id, text))
	return err
}

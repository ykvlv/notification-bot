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

// ensureUser makes sure a user row exists; if not, creates it with defaults.
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

// --- Generic helpers ---

func (r *Router) sendText(chatID int64, text string) {
	_, _ = r.bot.Send(tgbotapi.NewMessage(chatID, text))
}

func (r *Router) answerCallback(id, text string) error {
	_, err := r.bot.Request(tgbotapi.NewCallback(id, text))
	return err
}

// --- Core commands ---

func (r *Router) handleStart(ctx context.Context, chatID int64) {
	u, err := r.ensureUser(ctx, chatID)
	if err != nil {
		r.log.Error("ensureUser failed", zap.Error(err))
		r.sendText(chatID, "Profile initialization error. Please try again later.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, startText)
	msg.ReplyMarkup = mainMenuKeyboard(u.Enabled)
	_, _ = r.bot.Send(msg)
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
	enabledText := "✅ Enabled"
	if !u.Enabled {
		enabledText = "⏸ Paused"
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
		enabledText,
		next,
		u.Message,
	)

	msg := tgbotapi.NewMessage(chatID, body)
	msg.ReplyMarkup = mainMenuKeyboard(u.Enabled)
	_, _ = r.bot.Send(msg)
}

func (r *Router) handleSettings(ctx context.Context, chatID int64) {
	if _, err := r.ensureUser(ctx, chatID); err != nil {
		r.log.Error("ensureUser failed", zap.Error(err))
		r.sendText(chatID, "Error opening settings.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "What do you want to configure?")
	msg.ReplyMarkup = settingsInlineKeyboard()
	// Also refresh main keyboard under message:
	msg.ReplyMarkup = settingsInlineKeyboard()
	_, _ = r.bot.Send(msg)

	// Optionally, send a separate message with the main menu keyboard refreshed:
	// reply := tgbotapi.NewMessage(chatID, "Menu updated.")
	// reply.ReplyMarkup = mainMenuKeyboard(u.Enabled)
	// _, _ = r.bot.Send(reply)
}

// --- Interval flow ---

func (r *Router) askIntervalPresets(ctx context.Context, chatID int64, cbID string) {
	_ = r.answerCallback(cbID, "")
	msg := tgbotapi.NewMessage(chatID, "Choose an interval (or Custom to enter your own):")
	msg.ReplyMarkup = intervalPresetsKeyboard()
	_, _ = r.bot.Send(msg)
}

func (r *Router) handleIntervalCallback(ctx context.Context, chatID int64, data string, cbID string) {
	_ = r.answerCallback(cbID, "")
	if data == "interval:custom" {
		r.sendText(chatID, "Enter interval, e.g.: 30m, 1h, 1h30m, 90m")
		r.setPending(chatID, pendingInterval)
		return
	}
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

func (r *Router) updateInterval(ctx context.Context, chatID int64, d time.Duration) error {
	u, err := r.ensureUser(ctx, chatID)
	if err != nil {
		return err
	}
	u.IntervalSec = int(d.Seconds())
	// Recompute next_fire_at after interval change
	next := domain.NextFire(time.Now().UTC(), u)
	u.NextFireAt = &next
	return r.repo.UpsertUser(ctx, u)
}

// --- Free-form dispatcher (for all "Custom" inputs) ---

func (r *Router) handleFreeForm(ctx context.Context, chatID int64, text string) {
	switch r.getPending(chatID) {
	case pendingInterval:
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

	case pendingHours:
		r.clearPending(chatID)
		fromM, toM, err := domain.ParseActiveWindow(text)
		if err != nil {
			r.sendText(chatID, "Invalid format. Example: 09:00–21:00")
			return
		}
		if err := r.updateHours(ctx, chatID, fromM, toM); err != nil {
			r.log.Error("updateHours failed", zap.Error(err))
			r.sendText(chatID, "Could not save active hours.")
			return
		}
		r.sendText(chatID, "Active hours updated: "+domain.FormatMinutes(fromM)+"–"+domain.FormatMinutes(toM))

	case pendingTZ:
		r.clearPending(chatID)
		tz, err := domain.ValidateTZ(text)
		if err != nil {
			r.sendText(chatID, "Invalid timezone. Example: Europe/Moscow")
			return
		}
		if err := r.updateTZ(ctx, chatID, tz); err != nil {
			r.log.Error("updateTZ failed", zap.Error(err))
			r.sendText(chatID, "Could not save timezone.")
			return
		}
		r.sendText(chatID, "Timezone updated: "+tz)

	case pendingMessage:
		r.clearPending(chatID)
		if len(text) > 512 {
			r.sendText(chatID, "Too long. Please keep it under 512 characters.")
			return
		}
		u, err := r.ensureUser(ctx, chatID)
		if err != nil {
			r.log.Error("ensureUser failed", zap.Error(err))
			r.sendText(chatID, "Could not save message.")
			return
		}
		u.Message = text
		if err := r.repo.UpsertUser(ctx, u); err != nil {
			r.log.Error("save message failed", zap.Error(err))
			r.sendText(chatID, "Could not save message.")
			return
		}
		r.sendText(chatID, "Message updated.")

	default:
		// No pending flow: ignore free-form message
	}
}

// --- Active hours flow ---

func (r *Router) askHoursPresets(ctx context.Context, chatID int64, cbID string) {
	_ = r.answerCallback(cbID, "")
	msg := tgbotapi.NewMessage(chatID, "Choose active hours (or Custom):")
	msg.ReplyMarkup = hoursPresetsKeyboard()
	_, _ = r.bot.Send(msg)
}

func (r *Router) handleHoursCallback(ctx context.Context, chatID int64, data string, cbID string) {
	_ = r.answerCallback(cbID, "")
	if data == "hours:custom" {
		r.sendText(chatID, "Enter active hours as HH:MM–HH:MM (e.g., 09:00–21:00)")
		r.setPending(chatID, pendingHours)
		return
	}
	val := strings.TrimPrefix(data, "hours:")
	fromM, toM, err := domain.ParseActiveWindow(val)
	if err != nil {
		r.sendText(chatID, "Invalid format. Example: 09:00–21:00")
		return
	}
	if err := r.updateHours(ctx, chatID, fromM, toM); err != nil {
		r.log.Error("updateHours failed", zap.Error(err))
		r.sendText(chatID, "Could not save active hours.")
		return
	}
	r.sendText(chatID, "Active hours updated: "+domain.FormatMinutes(fromM)+"–"+domain.FormatMinutes(toM))
}

func (r *Router) updateHours(ctx context.Context, chatID int64, fromM, toM int) error {
	u, err := r.ensureUser(ctx, chatID)
	if err != nil {
		return err
	}
	u.ActiveFromM, u.ActiveToM = fromM, toM
	next := domain.NextFire(time.Now().UTC(), u)
	u.NextFireAt = &next
	return r.repo.UpsertUser(ctx, u)
}

// --- Timezone flow ---

func (r *Router) askTZPresets(ctx context.Context, chatID int64, cbID string) {
	_ = r.answerCallback(cbID, "")
	msg := tgbotapi.NewMessage(chatID, "Choose a timezone or enter your own (Region/City):")
	msg.ReplyMarkup = tzPresetsKeyboard()
	_, _ = r.bot.Send(msg)
}

func (r *Router) handleTZCallback(ctx context.Context, chatID int64, data string, cbID string) {
	_ = r.answerCallback(cbID, "")
	if data == "tz:custom" {
		r.sendText(chatID, "Enter timezone (e.g., Europe/Moscow):")
		r.setPending(chatID, pendingTZ)
		return
	}
	val := strings.TrimPrefix(data, "tz:")
	tz, err := domain.ValidateTZ(val)
	if err != nil {
		r.sendText(chatID, "Invalid timezone. Example: Europe/Moscow")
		return
	}
	if err := r.updateTZ(ctx, chatID, tz); err != nil {
		r.log.Error("updateTZ failed", zap.Error(err))
		r.sendText(chatID, "Could not save timezone.")
		return
	}
	r.sendText(chatID, "Timezone updated: "+tz)
}

func (r *Router) updateTZ(ctx context.Context, chatID int64, tz string) error {
	u, err := r.ensureUser(ctx, chatID)
	if err != nil {
		return err
	}
	u.TZ = tz
	next := domain.NextFire(time.Now().UTC(), u)
	u.NextFireAt = &next
	return r.repo.UpsertUser(ctx, u)
}

// --- Message flow ---

func (r *Router) askMessage(ctx context.Context, chatID int64, cbID string) {
	_ = r.answerCallback(cbID, "")
	r.sendText(chatID, "Send your reminder text in a single message (max 512 chars):")
	r.setPending(chatID, pendingMessage)
}

// --- Pause / Resume ---

func (r *Router) handlePause(ctx context.Context, chatID int64) {
	if err := r.repo.SetEnabled(ctx, chatID, false); err != nil {
		r.log.Error("pause failed", zap.Error(err))
		r.sendText(chatID, "Failed to pause.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "Paused ⏸")
	msg.ReplyMarkup = mainMenuKeyboard(false)
	_, _ = r.bot.Send(msg)
}

func (r *Router) handleResume(ctx context.Context, chatID int64) {
	if err := r.repo.SetEnabled(ctx, chatID, true); err != nil {
		r.log.Error("resume failed", zap.Error(err))
		r.sendText(chatID, "Failed to resume.")
		return
	}
	// Ensure next_fire_at is set after resuming.
	if u, err := r.ensureUser(ctx, chatID); err == nil {
		next := domain.NextFire(time.Now().UTC(), u)
		_ = r.repo.SetSchedule(ctx, chatID, next, nil)
	}
	msg := tgbotapi.NewMessage(chatID, "Resumed ✅")
	msg.ReplyMarkup = mainMenuKeyboard(true)
	_, _ = r.bot.Send(msg)
}

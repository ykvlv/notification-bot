package telegram

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/ykvlv/notification-bot/internal/store"
)

// Router holds bot API, logger and repo dependency.
type Router struct {
	bot  *tgbotapi.BotAPI
	log  *zap.Logger
	repo store.Repo

	state map[int64]string // chatID -> pending state
}

// NewRouter creates a new Telegram router.
func NewRouter(bot *tgbotapi.BotAPI, log *zap.Logger, repo store.Repo) *Router {
	return &Router{
		bot:   bot,
		log:   log,
		repo:  repo,
		state: make(map[int64]string),
	}
}

func (r *Router) setPending(chatID int64, s string) { r.state[chatID] = s }
func (r *Router) getPending(chatID int64) string    { return r.state[chatID] }
func (r *Router) clearPending(chatID int64)         { delete(r.state, chatID) }

// HandleUpdate routes a single update.
func (r *Router) HandleUpdate(ctx context.Context, upd tgbotapi.Update) {
	if upd.Message != nil {
		msg := upd.Message
		chatID := msg.Chat.ID
		text := strings.TrimSpace(msg.Text)

		switch {
		case strings.HasPrefix(text, "/start"):
			r.handleStart(ctx, chatID)
		case strings.HasPrefix(text, "/status"):
			r.handleStatus(ctx, chatID)
		case strings.HasPrefix(text, "/settings"):
			r.handleSettings(ctx, chatID)
		default:
			r.handleFreeForm(ctx, chatID, text)
		}
		return
	}

	if upd.CallbackQuery != nil {
		cb := upd.CallbackQuery
		data := cb.Data
		chatID := cb.Message.Chat.ID
		switch {
		case strings.HasPrefix(data, "set_interval"):
			r.askIntervalPresets(ctx, chatID, cb.ID)
		case strings.HasPrefix(data, "interval:"):
			r.handleIntervalCallback(ctx, chatID, data, cb.ID)
		default:
			// ignore unknown
		}
		return
	}
}

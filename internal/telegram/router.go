package telegram

import (
	"context"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/ykvlv/notification-bot/internal/store"
)

// Pending state keys used in conversational flows.
const (
	pendingInterval = "await_interval_text"
	pendingHours    = "await_hours_text"
	pendingTZ       = "await_tz_text"
	pendingMessage  = "await_message_text"
)

// Router wires Telegram updates to handlers and holds minimal in-memory state.
type Router struct {
	bot   *tgbotapi.BotAPI
	log   *zap.Logger
	repo  store.Repo
	state map[int64]string // chatID -> pending state
	mu    sync.RWMutex
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

// setPending sets a pending state for a chat (non-persistent, in-memory).
func (r *Router) setPending(chatID int64, s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state[chatID] = s
}

// getPending returns current pending state for a chat.
func (r *Router) getPending(chatID int64) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state[chatID]
}

// clearPending clears a pending state for a chat.
func (r *Router) clearPending(chatID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.state, chatID)
}

// HandleUpdate routes a single update to appropriate handler.
func (r *Router) HandleUpdate(ctx context.Context, upd tgbotapi.Update) {
	// Text messages
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
		case strings.HasPrefix(text, "/pause"):
			r.handlePause(ctx, chatID)
		case strings.HasPrefix(text, "/resume"):
			r.handleResume(ctx, chatID)
		default:
			// Free-form text used in "Custom" flows (interval/hours/tz/message)
			r.handleFreeForm(ctx, chatID, text)
		}
		return
	}

	// Callback queries (inline buttons)
	if upd.CallbackQuery != nil {
		cb := upd.CallbackQuery
		data := cb.Data
		chatID := cb.Message.Chat.ID

		switch {
		// Settings sections
		case data == "set_interval":
			r.askIntervalPresets(ctx, chatID, cb.ID)
		case strings.HasPrefix(data, "interval:"):
			r.handleIntervalCallback(ctx, chatID, data, cb.ID)

		case data == "set_hours":
			r.askHoursPresets(ctx, chatID, cb.ID)
		case strings.HasPrefix(data, "hours:"):
			r.handleHoursCallback(ctx, chatID, data, cb.ID)

		case data == "set_tz":
			r.askTZPresets(ctx, chatID, cb.ID)
		case strings.HasPrefix(data, "tz:"):
			r.handleTZCallback(ctx, chatID, data, cb.ID)

		case data == "set_msg":
			r.askMessage(ctx, chatID, cb.ID)

		default:
			// Unknown callback — ignore silently
		}
		return
	}
}

// SendMessage sends a plain text message to the given chat.
// This makes Router satisfy scheduler.Sender.
func (r *Router) SendMessage(chatID int64, text string) error {
	_, err := r.bot.Send(tgbotapi.NewMessage(chatID, text))
	return err
}

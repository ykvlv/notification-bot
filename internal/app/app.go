package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/ykvlv/notification-bot/internal/config"
	"github.com/ykvlv/notification-bot/internal/scheduler"
	"github.com/ykvlv/notification-bot/internal/store"
	"github.com/ykvlv/notification-bot/internal/telegram"
)

type App struct {
	cfg     config.Config
	log     *zap.Logger
	bot     *tgbotapi.BotAPI
	httpSrv *http.Server
	repo    store.Repo
	router  *telegram.Router
}

func New(cfg config.Config, log *zap.Logger) (*App, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, err
	}
	bot.Debug = false

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      mux,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	return &App{cfg: cfg, log: log, bot: bot, httpSrv: srv}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.log.Info("starting notification-bot",
		zap.String("mode", a.cfg.RunMode),
		zap.String("http", a.cfg.HTTPAddr),
		zap.String("bot_username", a.bot.Self.UserName),
	)

	// Open SQLite and run migrations.
	repo, err := store.OpenSQLite(ctx, a.cfg.DBPath)
	if err != nil {
		a.log.Error("sqlite open failed", zap.Error(err))
		return err
	}
	a.repo = repo
	a.log.Info("sqlite ready")

	// Router (Telegram handlers)
	a.router = telegram.NewRouter(a.bot, a.log, a.repo)

	// Start scheduler in background.
	sch := scheduler.New(a.repo, a.log, a.router)
	go sch.Run(ctx)

	// Start HTTP server.
	go func() {
		if err := a.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Error("http server error", zap.Error(err))
		}
	}()

	// Prepare Telegram updates channel.
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updCh := a.bot.GetUpdatesChan(u)

	// OS signals for graceful shutdown.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			a.log.Info("shutdown signal received")

			// Stop receiving Telegram updates and close channel.
			a.bot.StopReceivingUpdates()

			// Shutdown HTTP server.
			shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := a.httpSrv.Shutdown(shCtx)
			cancel()
			if err != nil {
				a.log.Warn("http server shutdown error", zap.Error(err))
			}

			// Close DB.
			if a.repo != nil {
				_ = a.repo.Close()
			}
			return nil

		case upd, ok := <-updCh:
			if !ok {
				// Channel closed by StopReceivingUpdates or internal error.
				a.log.Info("updates channel closed")
				// Proceed to graceful shutdown path.
				a.bot.StopReceivingUpdates()
				shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := a.httpSrv.Shutdown(shCtx)
				cancel()
				if err != nil {
					a.log.Warn("http server shutdown error", zap.Error(err))
				}
				if a.repo != nil {
					_ = a.repo.Close()
				}
				return nil
			}
			a.router.HandleUpdate(ctx, upd)
		}
	}
}

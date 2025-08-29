package app

import (
	"context"
	"errors"
	"github.com/ykvlv/notification-bot/internal/telegram"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/ykvlv/notification-bot/internal/config"
	"github.com/ykvlv/notification-bot/internal/store"
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
	)

	// Open SQLite and run migrations.
	repo, err := store.OpenSQLite(ctx, a.cfg.DBPath)
	if err != nil {
		a.log.Error("open sqlite failed", zap.Error(err))
		return err
	}
	a.repo = repo
	a.log.Info("sqlite ready")

	a.router = telegram.NewRouter(a.bot, a.log, a.repo)

	go func() {
		if err := a.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Error("http server error", zap.Error(err))
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updCh := a.bot.GetUpdatesChan(u)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			a.log.Info("shutdown signal received")

			// Create a short-lived shutdown context and cancel it immediately after use.
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

		case upd := <-updCh:
			a.router.HandleUpdate(ctx, upd)
		}
	}
}

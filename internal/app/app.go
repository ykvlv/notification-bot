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
)

type App struct {
	cfg     config.Config
	log     *zap.Logger
	bot     *tgbotapi.BotAPI
	httpSrv *http.Server
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
			return nil

		case <-updCh:
			// Stage 0: ignore updates
		}
	}
}

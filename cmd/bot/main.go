package main

import (
	"context"
	"os"

	"go.uber.org/zap"

	"github.com/ykvlv/notification-bot/internal/app"
	"github.com/ykvlv/notification-bot/internal/config"
	"github.com/ykvlv/notification-bot/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// No logger yet; exit immediately.
		// We intentionally ignore Fprintf errors to avoid shadowing the real cause.
		_, _ = os.Stderr.WriteString("config error: " + err.Error() + "\n")
		os.Exit(2)
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		_, _ = os.Stderr.WriteString("logger init error: " + err.Error() + "\n")
		os.Exit(2)
	}
	// Ensure logger flush; ignore sync error (common on some platforms).
	defer func() { _ = log.Sync() }()

	application, err := app.New(cfg, log)
	if err != nil {
		log.Fatal("app init failed", zap.Error(err))
	}

	if err := application.Run(context.Background()); err != nil {
		log.Fatal("app run failed", zap.Error(err))
	}
}

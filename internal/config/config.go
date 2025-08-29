package config

import "github.com/kelseyhightower/envconfig"

// Config holds application configuration loaded from environment variables.
type Config struct {
	BotToken  string `envconfig:"BOT_TOKEN" required:"true"`
	DBPath    string `envconfig:"DB_PATH" default:"./data/notification.db"`
	DefaultTZ string `envconfig:"DEFAULT_TZ" default:"Europe/Moscow"`
	RunMode   string `envconfig:"RUN_MODE" default:"polling"` // polling|webhook (MVP: polling)
	LogLevel  string `envconfig:"LOG_LEVEL" default:"info"`   // debug|info|warn|error
	HTTPAddr  string `envconfig:"HTTP_ADDR" default:":8080"`  // healthz (future-proof)
}

// Load reads environment variables into Config.
func Load() (Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

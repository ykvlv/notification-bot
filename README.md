# notification-bot

A simple Telegram reminder bot written in Go.  
Sends motivational (or any) reminders at a configurable interval, within active hours, respecting user timezone.

## Features
- Per-user settings (stored in embedded SQLite):
	- Interval (e.g., `30m`, `1h30m`, `24h`)
	- Active hours window (e.g., `09:00–21:00`, supports wrap-around like `22:00–02:00`)
	- Timezone (IANA, e.g., `Europe/Moscow`)
	- Custom message
	- Pause/Resume
- Automatic scheduling (`next_fire_at`) and dispatch loop.
- `/examples` — sends bundled MP3 files you can set as custom notification sounds in Telegram.

## Quick start

```bash
go mod tidy
export BOT_TOKEN=123456:AA...

make build
./bin/notification-bot
```

Healthcheck: GET http://localhost:8080/healthz → 200

## Commands
- `/start` — initialize profile and show menu
- `/status` — show current settings (interval, active hours, TZ, enabled, next, message)
- `/settings` — configure interval, hours, timezone, message (inline UI)
- `/pause` / `/resume` — toggle scheduling
- `/examples` — receive bundled MP3 examples

## Configuration (env)
- `BOT_TOKEN` — Telegram Bot API token (required)
- `DB_PATH` — path to SQLite file (default `./data/notification.db`)
- `DEFAULT_TZ` — default timezone for new users (default `Europe/Moscow`)
- `HTTP_ADDR` — health endpoint address (default `:8080`)
- `LOG_LEVEL` — `debug|info|warn|error` (default `info`)

## Storage
- SQLite (via `modernc.org/sqlite`)
- Table: `users` with fields: `chat_id`, `enabled`, `tz`, `interval_sec`, `active_from_m`, `active_to_m`, `message`, `next_fire_at`, `last_sent_at`, `created_at`.
- Migrations via `go:embed`.

## Build
- `make build` — build static binary to `bin/notification-bot`
- `make run` — run with envs
- `make tidy` — tidy modules

## License
MIT. See `LICENSE.md`.

---

☕️ This bot was fully vibe-coded in ~3 hours, powered by a liter of tea, a pack of cookies, and ChatGPT 5.

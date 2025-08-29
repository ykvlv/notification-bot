# AGENTS.md — Guide for AI Assistants

## Project Overview
This repository contains **notification-bot**, a Telegram bot written in Go.  
Its purpose is to send motivational reminders to users at configurable intervals.

### Requirements & Goals
- **Embedded storage**: use SQLite (no external DB).
- **User settings**:
    - Interval in human-readable format (not cron).
    - Active hours and timezone support.
    - Custom motivational message.
    - Option to receive example MP3 sounds to use as Telegram custom notification tones.
- **Bot behavior**:
    - Works per-user (each chat_id has independent config).
    - Runs without external dependencies (single binary + SQLite file).
    - On restart, schedules are restored from storage.
- **Commands**:
    - `/start`, `/help`, `/status`, `/settings`, `/pause`, `/resume`, `/examples`.
- **Non-functional requirements**:
    - Simple UX (inline buttons, human-friendly input).
    - Reliable: atomic persistence, safe shutdown.
    - Open source, permissive license (MIT/CC0).

---

## Stage 0 — Bootstrap
Implemented **initial project skeleton**:

- Project name: `notification-bot`
- Owner: `ykvlv`
- Default timezone: `Europe/Moscow`
- Repository layout:

```shell
notification-bot/
├─ cmd/bot/main.go        # entrypoint
├─ internal/
│  ├─ app/                # app lifecycle, signals, Telegram polling
│  ├─ config/             # env-based configuration
│  └─ logger/             # zap logger
├─ .env.example           # example config
├─ .gitignore             # ignores bin, data, env files
├─ go.mod / go.sum        # Go module
└─ Makefile               # simple build/run/tidy targets
```

- Implemented:
- Config loader (`envconfig`), no CLI flags, defaults set.
- JSON logger with `zap`.
- App skeleton:
    - Graceful shutdown on SIGTERM/SIGINT.
    - HTTP `/healthz` endpoint (future-proof).
    - Telegram bot long polling (updates ignored in Stage 0).
- Error handling fixed (no unhandled errors, no leaking defers).
- Build/test flow:
- `make build` → binary `bin/notification-bot`
- `make run` → run with env vars
- `make tidy` → sync dependencies

**Status:** Stage 0 complete. Bot starts, logs, serves `/healthz`, connects to Telegram API, and shuts down gracefully.

## Stage 1 — SQLite & Repository

- Added embedded SQLite storage using `modernc.org/sqlite`.
- Implemented migration runner with `go:embed`. On startup, it automatically:
  - Ensures `data/notification.db` file exists.
  - Applies PRAGMAs (`journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`, `foreign_keys=ON`).
  - Executes SQL migrations from `internal/store/migrations/`.
- Initial schema created:
  - `users` table with fields:
    - `chat_id` (primary key)
    - `created_at` (UTC unix seconds)
    - `enabled` (pause/resume flag)
    - `tz` (IANA timezone string, default `Europe/Moscow`)
    - `interval_sec` (notification interval, in seconds)
    - `active_from_m` / `active_to_m` (active hours in minutes from midnight)
    - `message` (reminder text)
    - `next_fire_at` (unix seconds, nullable)
    - `last_sent_at` (unix seconds, nullable)
  - Index `idx_users_nextfire` on `next_fire_at` for fast scheduler queries.
- Repository implemented (`internal/store/sqlite.go`):
  - `UpsertUser` — insert or update user settings.
  - `GetUser` — fetch settings by chat_id.
  - `ListDue` — list users due for notification at `<= now`.
  - `SetSchedule` — update `next_fire_at` and `last_sent_at`.
  - `SetEnabled` — enable/disable user.
  - `Close` — close DB connection.
- App startup now opens DB and logs `"sqlite ready"`.

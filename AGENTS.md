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

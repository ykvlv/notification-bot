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

## Stage 2 — Parsers & Basic Telegram Handlers

- Added parsers and validators (`internal/domain/parse.go`):
  - **Duration** parser: accepts `30m`, `1h`, `1h30m`, `90m`, `24h` etc. → validates (10m ≤ d ≤ 72h).
  - **Active hours** parser: accepts `HH:MM–HH:MM` or `HH:MM-HH:MM` → returns minutes since midnight.
  - **Timezone** validator: checks IANA TZ (via `time.LoadLocation`).
  - **Formatters**: `FormatMinutes` → `HH:MM`, `LocalizeTime` → localized `HH:MM`.

- Added Telegram UI layer (`internal/telegram/`):
  - **texts.go**: static texts + keyboards (main menu, settings menu, interval presets).
  - **router.go**: routes updates (messages, callbacks) to handlers.
  - **handlers.go**:
    - `/start`: ensures user exists (with defaults) and shows main menu.
    - `/status`: displays current settings (interval, active hours, TZ, enabled flag, next fire time).
    - `/settings`: shows inline keyboard for changing settings (Interval, Active hours, TZ, Message).
    - Interval flow:
      - Preset buttons: 30m, 1h, 2h, 3h, 4h, 6h, 8h, 12h, 24h.
      - Custom input: expects next free-form message, validates duration.
      - Updates DB via `repo.UpsertUser`.

- Added simple **in-memory pending state** in router (chatID → "await_interval_text") to support conversational flows.
  - To be moved into DB later if needed.

- App (`app.go`) now wires `telegram.Router` into update loop:
  - `router.HandleUpdate(ctx, upd)` called for every update.
  - Repo ensures user row on first `/start`.

**Status:**  
At this stage, the bot can:
- Initialize a user with defaults.
- Show current settings with `/status`.
- Open settings with `/settings`.
- Update interval via presets or custom input.

## Stage 3 — Active Hours, Timezone, Message, NextFire

- Implemented **NextFire** computation (`internal/domain/schedule.go`):
  - Respects user interval, active hours window (including wrap-around windows like 22:00–02:00), and IANA timezone.
  - If outside window, jumps to next window start; otherwise advances by interval and clamps to next valid window.

- Extended Telegram handlers:
  - **Active hours** (`set_hours`): presets (08:00–22:00, 09:00–21:00, 22:00–02:00) + Custom input (`HH:MM–HH:MM`).
  - **Timezone** (`set_tz`): presets (Europe/Moscow, Europe/Tallinn, Asia/Almaty, UTC) + Custom IANA input.
  - **Message** (`set_msg`): free-form text (<= 512 chars). `/status` now displays the current message.
  - **Pause/Resume**: `/pause`, `/resume` toggle `enabled` and recompute next trigger.
    - Main menu keyboard now shows a single dynamic button: either `/pause` or `/resume` depending on the current state.
  - All setting changes recompute and persist `next_fire_at`.

- Router pending states extended:
  - `await_hours_text`, `await_tz_text`, `await_message_text`.

**Status:**  
The bot can now fully configure **interval**, **active hours**, **timezone**, and **message**.  
It computes and stores `next_fire_at`. Actual dispatching of notifications will be implemented in Stage 4.

## Stage 4 — Scheduler & Notification Dispatch

- Implemented **scheduler** (`internal/scheduler/scheduler.go`):
  - Runs a background loop every 30 seconds.
  - Calls `repo.ListDue(now)` to fetch users with `next_fire_at <= now`.
  - Sends `u.Message` to each due user via Telegram.
  - Updates `last_sent_at` and recomputes `next_fire_at` using `domain.NextFire`.

- Extended **telegram.Router**:
  - Added `SendMessage(chatID, text)` method so Router implements `scheduler.Sender`.

- Integrated scheduler into `app.Run`:
  - Created a scheduler instance with repo, logger, and router.
  - Launched it in a goroutine alongside the update loop.
  - Scheduler stops gracefully on context cancel.

- Improved **graceful shutdown**:
  - Added `bot.StopReceivingUpdates()` to stop Telegram polling.
  - Updates channel (`updCh`) is handled safely if closed.
  - Ensures HTTP server and DB are closed on shutdown.

**Status:**  
The bot now automatically **dispatches notifications** at the configured interval, respecting active hours and user timezone.  
After each message, the scheduler updates scheduling fields in DB to ensure continuous reminders.

## Stage 5 — Audio Examples & Initial NextFire

- **Fix:**
  - When a new user is created via `/start`, the bot now computes and persists `next_fire_at` immediately using `domain.NextFire`.
  - As a result, `/status` for a fresh user shows a valid **Next** time instead of `—`.

- **Audio Examples:**
  - Added bundled MP3 examples under `assets/` and embedded them via `go:embed`.
  - Files: `Motivation.mp3`, `Do_It.mp3`, `Gym.mp3` (can be extended with more meme sounds later).
  - New command `/examples` and new button **"Audio examples"** in `/settings`.
  - Handler sends all embedded MP3s to the user as Telegram audio files.
  - No DB fields are used; users can set these MP3s as **custom notification sounds** in Telegram client settings.

- **UX:**
  - `/start` message updated to mention `/examples` and the ability to receive ready-made MP3 sounds.
  - After sending audio, bot reminds users: *"Open Telegram notification settings to set this as a custom sound."*

**Status:**  
The bot now not only manages intervals, hours, timezone, and messages with automatic notifications,  
but also provides **ready-made audio examples** for quick use as Telegram custom sounds.

## Stage 6 — UX Polishing & Docs

- Improved user-facing texts:
  - Emojis in `/start`, `/status`, and settings menus.
  - Clearer headings and confirmation messages.
- Added **Back** inline action in interval/hours/timezone flows.
- Start message now highlights `/examples` and explains MP3 usage.
- README.md written:
  - Features, commands, configuration, storage, build, license.
  - With a playful note: *“☕️ This bot was fully vibe-coded in ~3 hours, powered by a liter of tea, a pack of cookies, and ChatGPT 5.”*

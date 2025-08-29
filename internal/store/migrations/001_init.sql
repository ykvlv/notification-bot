-- schema init
CREATE TABLE IF NOT EXISTS users (
    chat_id        INTEGER PRIMARY KEY,
    created_at     INTEGER NOT NULL,
    enabled        INTEGER NOT NULL DEFAULT 1,
    tz             TEXT NOT NULL DEFAULT 'Europe/Moscow',
    interval_sec   INTEGER NOT NULL,
    active_from_m  INTEGER NOT NULL,
    active_to_m    INTEGER NOT NULL,
    message        TEXT NOT NULL,
    next_fire_at   INTEGER,
    last_sent_at   INTEGER
);


CREATE INDEX IF NOT EXISTS idx_users_nextfire ON users(next_fire_at);

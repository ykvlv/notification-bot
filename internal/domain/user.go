package domain

import "time"

// User represents per-chat notification settings and schedule.
type User struct {
	ChatID      int64
	Enabled     bool
	TZ          string
	IntervalSec int        // notification interval in seconds
	ActiveFromM int        // minutes from midnight (0..1439)
	ActiveToM   int        // minutes from midnight (0..1439)
	Message     string     //
	NextFireAt  *time.Time // UTC, nullable
	LastSentAt  *time.Time // UTC, nullable
	CreatedAt   time.Time  // UTC
}

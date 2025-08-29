package domain

import "time"

// InWindow returns true if local time (minutes since midnight) is inside active window.
// Supports wrap-around windows like 22:00–02:00 (fromM > toM).
func InWindow(localM, fromM, toM int) bool {
	if fromM == toM {
		return false // zero-length window
	}
	if fromM < toM {
		return localM >= fromM && localM < toM
	}
	// wrap: [from..1440) U [0..to)
	return localM >= fromM || localM < toM
}

// NextFire computes the next fire time in UTC for a user given current time in UTC.
// It advances by interval starting from "now", constrained by the active window in user's TZ.
// If now is outside the window, it jumps to the next window start.
// If interval steps land outside the window, they advance until within the window or roll to next day's start.
func NextFire(nowUTC time.Time, u *User) time.Time {
	loc, err := time.LoadLocation(u.TZ)
	if err != nil {
		loc = time.UTC
	}
	interval := time.Duration(u.IntervalSec) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}

	// Represent "now" in user's local date/time.
	localNow := nowUTC.In(loc)
	// Minutes since midnight for localNow.
	localM := localNow.Hour()*60 + localNow.Minute()

	// Helper to construct local date at given minutes (same date as localNow).
	makeLocalAt := func(base time.Time, mins int) time.Time {
		h := mins / 60
		m := mins % 60
		return time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, base.Location())
	}

	// If outside window → jump to next window start (today or tomorrow depending on wrap and position).
	if !InWindow(localM, u.ActiveFromM, u.ActiveToM) {
		var start time.Time
		if u.ActiveFromM < u.ActiveToM {
			// normal window (e.g., 09:00–22:00)
			if localM < u.ActiveFromM {
				start = makeLocalAt(localNow, u.ActiveFromM)
			} else {
				// already past today's window → start tomorrow
				start = makeLocalAt(localNow.Add(24*time.Hour), u.ActiveFromM)
			}
		} else {
			// wrap window (e.g., 22:00–02:00): window is [from..24h) U [0..to)
			if localM < u.ActiveToM {
				// early morning before toM → still in window actually; but InWindow already false here → means fromM==toM? (handled above)
				// for safety, jump to next possible start = makeLocalAt(today, fromM) if in evening, else today 00:00 is inside
				// Simpler: jump to today's fromM if later today, else use now if entering soon.
				start = makeLocalAt(localNow, u.ActiveFromM) // may be in past; if so, add 24h below
				if !start.After(localNow) {
					start = start.Add(24 * time.Hour)
				}
			} else if localM >= u.ActiveFromM {
				// evening but outside? (shouldn't happen since evening >= from is inside); still align to now
				start = makeLocalAt(localNow, u.ActiveFromM)
				if !start.After(localNow) {
					start = localNow // already in window start segment
				}
			} else {
				// midday (between to..from) → next start at fromM today
				start = makeLocalAt(localNow, u.ActiveFromM)
			}
		}
		// Convert to UTC and return.
		return start.UTC()
	}

	// Inside window: advance by interval until we land within a window boundary.
	nextLocal := localNow.Add(interval)
	for attempts := 0; attempts < 48; attempts++ { // cap attempts to avoid infinite loops
		nm := nextLocal.Hour()*60 + nextLocal.Minute()
		if InWindow(nm, u.ActiveFromM, u.ActiveToM) {
			return nextLocal.UTC()
		}
		// If we crossed out of window, jump to next window start.
		if u.ActiveFromM < u.ActiveToM {
			// normal window → next day at fromM
			nextLocal = time.Date(nextLocal.Year(), nextLocal.Month(), nextLocal.Day(), 0, 0, 0, 0, nextLocal.Location())
			nextLocal = nextLocal.Add(24 * time.Hour)
			nextLocal = nextLocal.Add(time.Duration(u.ActiveFromM) * time.Minute)
		} else {
			// wrap window: determine whether to jump to today's fromM or tomorrow's fromM
			candidate := time.Date(nextLocal.Year(), nextLocal.Month(), nextLocal.Day(), 0, 0, 0, 0, nextLocal.Location())
			candidate = candidate.Add(time.Duration(u.ActiveFromM) * time.Minute)
			if !candidate.After(nextLocal) {
				candidate = candidate.Add(24 * time.Hour)
			}
			nextLocal = candidate
		}
	}
	// Fallback: return next day window start.
	fallback := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc).
		Add(24 * time.Hour).
		Add(time.Duration(u.ActiveFromM) * time.Minute)
	return fallback.UTC()
}

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
// Slots are anchored to the beginning of the active window in the user's TZ.
// Inside a window, the next time is the nearest slot strictly after now that equals:
//
//	windowStart + k*interval
//
// If that slot falls outside the current window, schedule the start of the next window.
// If now is outside the window, schedule at the next window start.
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
	localM := localNow.Hour()*60 + localNow.Minute()

	// Helper: construct local date at given minutes (same date as base).
	makeLocalAt := func(base time.Time, mins int) time.Time {
		h := mins / 60
		m := mins % 60
		return time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, base.Location())
	}

	// Helper: get current window bounds [start, end) that contains localNow for wrap/normal.
	// Returns ok=false if now is outside any current window; then caller decides the next start.
	windowBounds := func(now time.Time, fromM, toM int) (start, end time.Time, ok bool) {
		if fromM == toM {
			return time.Time{}, time.Time{}, false
		}
		if fromM < toM {
			start = makeLocalAt(now, fromM)
			end = makeLocalAt(now, toM)
			if now.Before(start) || !now.Before(end) { // outside
				return time.Time{}, time.Time{}, false
			}
			return start, end, true
		}
		// wrap window: [from..24h) U [0..to)
		localM := now.Hour()*60 + now.Minute()
		if localM >= fromM { // evening segment today
			start = makeLocalAt(now, fromM)
			end = makeLocalAt(now.Add(24*time.Hour), toM)
			return start, end, true
		}
		if localM < toM { // early morning segment today (window started yesterday)
			start = makeLocalAt(now.Add(-24*time.Hour), fromM)
			end = makeLocalAt(now, toM)
			return start, end, true
		}
		return time.Time{}, time.Time{}, false
	}

	// If outside window → jump to the next window start (today or tomorrow depending on position & wrap).
	if !InWindow(localM, u.ActiveFromM, u.ActiveToM) {
		if u.ActiveFromM < u.ActiveToM {
			// normal window: next start today if before from, else tomorrow
			if localM < u.ActiveFromM {
				return makeLocalAt(localNow, u.ActiveFromM).UTC()
			}
			return makeLocalAt(localNow.Add(24*time.Hour), u.ActiveFromM).UTC()
		}
		// wrap window: next start at today's fromM if we're between to..from; if we're after fromM, we're actually inside (handled above)
		return makeLocalAt(localNow, u.ActiveFromM).UTC()
	}

	// Inside window: anchor to window start and pick the next aligned slot strictly after now.
	start, end, ok := windowBounds(localNow, u.ActiveFromM, u.ActiveToM)
	if !ok {
		// Safety: if detection failed, fall back to next start
		if u.ActiveFromM < u.ActiveToM {
			return makeLocalAt(localNow.Add(24*time.Hour), u.ActiveFromM).UTC()
		}
		return makeLocalAt(localNow, u.ActiveFromM).UTC()
	}

	elapsed := localNow.Sub(start)
	// We want the first slot strictly after now → add one full interval beyond the last passed boundary.
	slots := elapsed / interval
	nextLocal := start.Add((slots + 1) * interval)

	// If the computed slot falls outside the current window, schedule the start of the next window.
	if nextLocal.After(end) {
		if u.ActiveFromM < u.ActiveToM {
			return makeLocalAt(localNow.Add(24*time.Hour), u.ActiveFromM).UTC()
		}
		// For wrap window, the next window start is on the day of 'end' at fromM
		nextStart := makeLocalAt(end, u.ActiveFromM)
		return nextStart.UTC()
	}

	return nextLocal.UTC()
}

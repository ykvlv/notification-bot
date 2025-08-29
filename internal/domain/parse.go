package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrEmptyDuration   = errors.New("empty duration")
	ErrInvalidDuration = errors.New("invalid duration")
	ErrTooSmall        = errors.New("duration too small")
	ErrTooLarge        = errors.New("duration too large")
)

// ParseDurationHuman parses human-friendly durations like "30m", "1h30m", "90m", "2h".
// Constraints (MVP): 10m <= d <= 72h.
func ParseDurationHuman(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, ErrEmptyDuration
	}
	// Accept formats: "90m", "2h", "1h30m"
	var total time.Duration

	// Simple case: plain number means minutes (e.g., "90")
	if isAllDigits(s) {
		mins, _ := strconv.Atoi(s)
		total = time.Duration(mins) * time.Minute
	} else {
		re := regexp.MustCompile(`(?i)(\d+)\s*h`)
		mh := re.FindStringSubmatch(s)
		if len(mh) == 2 {
			h, _ := strconv.Atoi(mh[1])
			total += time.Duration(h) * time.Hour
		}
		re = regexp.MustCompile(`(?i)(\d+)\s*m`)
		mm := re.FindStringSubmatch(s)
		if len(mm) == 2 {
			m, _ := strconv.Atoi(mm[1])
			total += time.Duration(m) * time.Minute
		}
		// Fallback: if none matched and not all digits, it's invalid
		if total == 0 && !(strings.Contains(s, "h") || strings.Contains(s, "m")) {
			return 0, fmt.Errorf("%w: %s", ErrInvalidDuration, s)
		}
	}

	if total < 10*time.Minute {
		return 0, fmt.Errorf("%w: min 10m", ErrTooSmall)
	}
	if total > 72*time.Hour {
		return 0, fmt.Errorf("%w: max 72h", ErrTooLarge)
	}
	return total, nil
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// ParseActiveWindow parses "HH:MM–HH:MM" or "HH:MM-HH:MM" into minutes since midnight.
func ParseActiveWindow(s string) (fromM, toM int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, errors.New("empty window")
	}
	sep := "–"
	if strings.Contains(s, "-") && !strings.Contains(s, "–") {
		sep = "-"
	}
	parts := strings.Split(s, sep)
	if len(parts) != 2 {
		return 0, 0, errors.New("expected format HH:MM–HH:MM")
	}
	fromM, err = parseHHMM(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("from: %w", err)
	}
	toM, err = parseHHMM(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("to: %w", err)
	}
	if fromM < 0 || fromM > 1439 || toM < 0 || toM > 1439 {
		return 0, 0, errors.New("time out of range")
	}
	return fromM, toM, nil
}

func parseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, errors.New("expected HH:MM")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, errors.New("invalid hour")
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, errors.New("invalid minute")
	}
	return h*60 + m, nil
}

// ValidateTZ checks that the tz is a valid IANA location.
func ValidateTZ(tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", err
	}
	return loc.String(), nil
}

// FormatMinutes returns HH:MM for minutes since midnight (00:00..23:59).
func FormatMinutes(mins int) string {
	if mins < 0 {
		mins = 0
	}
	h := mins / 60
	m := mins % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

// LocalizeTime formats t in user's timezone as HH:MM.
func LocalizeTime(t time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", err
	}
	lt := t.In(loc)
	return lt.Format("15:04"), nil
}

package domain

import (
	"testing"
	"time"
)

// helper: build a time in given tz and return its UTC
func mustLocalUTC(t *testing.T, tz string, y int, m time.Month, d, hh, mm int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}
	lt := time.Date(y, m, d, hh, mm, 0, 0, loc)
	return lt.UTC()
}

func TestNextFire_AnchoredNormalWindow(t *testing.T) {
	u := &User{
		ChatID:      1,
		Enabled:     true,
		TZ:          "Europe/Moscow",
		IntervalSec: int((2 * time.Hour).Seconds()),
		ActiveFromM: 9 * 60,
		ActiveToM:   23 * 60,
		Message:     "",
	}
	// 2025-05-05 19:46 MSK → expect 21:00 MSK
	nowUTC := mustLocalUTC(t, u.TZ, 2025, time.May, 5, 19, 46)
	next := NextFire(nowUTC, u)
	got, err := LocalizeTime(next, u.TZ)
	if err != nil {
		t.Fatalf("localize: %v", err)
	}
	want := "21:00"
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestNextFire_BeforeWindowStartsToday(t *testing.T) {
	u := &User{
		ChatID:      1,
		Enabled:     true,
		TZ:          "Europe/Moscow",
		IntervalSec: int((30 * time.Minute).Seconds()),
		ActiveFromM: 9 * 60,
		ActiveToM:   23 * 60,
	}
	// 07:00 local → expect 09:00 local
	nowUTC := mustLocalUTC(t, u.TZ, 2025, time.May, 6, 7, 0)
	next := NextFire(nowUTC, u)
	got, _ := LocalizeTime(next, u.TZ)
	if got != "09:00" {
		t.Fatalf("want 09:00, got %s", got)
	}
}

func TestNextFire_WrapWindow_EveningSegment(t *testing.T) {
	u := &User{
		TZ:          "Europe/Moscow",
		IntervalSec: int((2 * time.Hour).Seconds()),
		ActiveFromM: 22 * 60,
		ActiveToM:   2 * 60,
	}
	// 23:15 local within wrap window → expect 00:00 next day
	nowUTC := mustLocalUTC(t, u.TZ, 2025, time.May, 7, 23, 15)
	next := NextFire(nowUTC, u)
	got, _ := LocalizeTime(next, u.TZ)
	if got != "00:00" {
		t.Fatalf("want 00:00, got %s", got)
	}
}

func TestNextFire_WrapWindow_MorningSegment(t *testing.T) {
	u := &User{
		TZ:          "Europe/Moscow",
		IntervalSec: int((30 * time.Minute).Seconds()),
		ActiveFromM: 22 * 60,
		ActiveToM:   2 * 60,
	}
	// 01:30 local within wrap window → expect 02:00
	nowUTC := mustLocalUTC(t, u.TZ, 2025, time.May, 8, 1, 30)
	next := NextFire(nowUTC, u)
	got, _ := LocalizeTime(next, u.TZ)
	if got != "02:00" {
		t.Fatalf("want 02:00, got %s", got)
	}
}

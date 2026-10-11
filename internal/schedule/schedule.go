// Package schedule holds the booking-time rules (opening hours, slot grid,
// overlap checks). It depends only on the standard library so it is easy to test.
package schedule

import (
	"fmt"
	"time"
)

// Range is a half-open time interval [Start, End).
type Range struct {
	Start time.Time
	End   time.Time
}

// Schedule describes when the shop takes bookings. All hours are in Loc.
type Schedule struct {
	Loc         *time.Location
	OpenHour    int // inclusive, 0-23
	CloseHour   int // exclusive end of the last booking, 1-24
	SlotMinutes int // booking grid step, must divide 60
}

func New(tz string, openHour, closeHour, slotMinutes int) (*Schedule, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", tz, err)
	}
	if openHour < 0 || openHour > 23 {
		return nil, fmt.Errorf("open hour must be 0-23, got %d", openHour)
	}
	if closeHour <= openHour || closeHour > 24 {
		return nil, fmt.Errorf("close hour must be greater than open hour and at most 24, got %d", closeHour)
	}
	if slotMinutes <= 0 || 60%slotMinutes != 0 {
		return nil, fmt.Errorf("slot minutes must divide 60 (e.g. 15, 30, 60), got %d", slotMinutes)
	}
	return &Schedule{Loc: loc, OpenHour: openHour, CloseHour: closeHour, SlotMinutes: slotMinutes}, nil
}

// ParseDate parses "YYYY-MM-DD" as midnight in the shop's timezone.
func (s *Schedule) ParseDate(v string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", v, s.Loc)
}

// Slots returns every free start time on the given day for a service of length dur.
// Past times and times overlapping a busy range are skipped.
func (s *Schedule) Slots(day time.Time, dur time.Duration, busy []Range, now time.Time) []time.Time {
	open, closing := s.hoursOn(day)
	step := time.Duration(s.SlotMinutes) * time.Minute

	out := []time.Time{}
	for t := open; !t.Add(dur).After(closing); t = t.Add(step) {
		if !t.After(now) {
			continue
		}
		if overlapsAny(t, t.Add(dur), busy) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// ValidStart reports whether start is a legal booking start for a service of length dur:
// on the slot grid, inside opening hours, and finishing before closing time.
func (s *Schedule) ValidStart(start time.Time, dur time.Duration) bool {
	local := start.In(s.Loc)
	if local.Second() != 0 || local.Nanosecond() != 0 || local.Minute()%s.SlotMinutes != 0 {
		return false
	}
	open, closing := s.hoursOn(local)
	return !start.Before(open) && !start.Add(dur).After(closing)
}

// DayBounds returns [00:00, next 00:00) of the day containing t, in the shop's timezone.
func (s *Schedule) DayBounds(t time.Time) (time.Time, time.Time) {
	y, m, d := t.In(s.Loc).Date()
	from := time.Date(y, m, d, 0, 0, 0, 0, s.Loc)
	return from, time.Date(y, m, d+1, 0, 0, 0, 0, s.Loc)
}

func (s *Schedule) hoursOn(day time.Time) (open, closing time.Time) {
	y, m, d := day.In(s.Loc).Date()
	open = time.Date(y, m, d, s.OpenHour, 0, 0, 0, s.Loc)
	closing = time.Date(y, m, d, s.CloseHour, 0, 0, 0, s.Loc)
	return open, closing
}

func overlapsAny(start, end time.Time, busy []Range) bool {
	for _, b := range busy {
		if b.Start.Before(end) && b.End.After(start) {
			return true
		}
	}
	return false
}

package schedule

import (
	"testing"
	"time"
)

func newTestSchedule(t *testing.T) *Schedule {
	t.Helper()
	s, err := New("Asia/Bangkok", 9, 18, 30)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func at(s *Schedule, day string, hour, min int) time.Time {
	d, _ := s.ParseDate(day)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, s.Loc)
}

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name              string
		tz                string
		open, close, slot int
		wantErr           bool
	}{
		{"ok", "Asia/Bangkok", 9, 18, 30, false},
		{"bad tz", "Mars/Base", 9, 18, 30, true},
		{"close before open", "Asia/Bangkok", 18, 9, 30, true},
		{"slot not dividing 60", "Asia/Bangkok", 9, 18, 25, true},
		{"close after 24", "Asia/Bangkok", 9, 25, 30, true},
	}
	for _, c := range cases {
		_, err := New(c.tz, c.open, c.close, c.slot)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestSlotsFullDay(t *testing.T) {
	s := newTestSchedule(t)
	day, _ := s.ParseDate("2030-01-10")
	now := at(s, "2030-01-01", 0, 0)

	// 60 min service, 09:00-18:00, 30 min grid -> starts 09:00 ... 17:00 = 17 slots
	got := s.Slots(day, 60*time.Minute, nil, now)
	if len(got) != 17 {
		t.Fatalf("want 17 slots, got %d", len(got))
	}
	if !got[0].Equal(at(s, "2030-01-10", 9, 0)) {
		t.Errorf("first slot = %v", got[0])
	}
	if !got[len(got)-1].Equal(at(s, "2030-01-10", 17, 0)) {
		t.Errorf("last slot = %v", got[len(got)-1])
	}
}

func TestSlotsSkipBusyAndPast(t *testing.T) {
	s := newTestSchedule(t)
	day, _ := s.ParseDate("2030-01-10")
	busy := []Range{{Start: at(s, "2030-01-10", 10, 0), End: at(s, "2030-01-10", 11, 0)}}

	// "now" is 09:30 on the same day: 09:00 and 09:30 are not bookable.
	now := at(s, "2030-01-10", 9, 30)
	got := s.Slots(day, 60*time.Minute, busy, now)

	for _, slot := range got {
		if !slot.After(now) {
			t.Errorf("slot %v is not in the future", slot)
		}
		end := slot.Add(60 * time.Minute)
		if slot.Before(busy[0].End) && end.After(busy[0].Start) {
			t.Errorf("slot %v overlaps the busy range", slot)
		}
	}
	// 10:00 would overlap; 09:00 start is past; 11:00 is the first free slot after the busy range,
	// and 09:30-10:30 overlaps too, so the first result must be 11:00.
	if len(got) == 0 || !got[0].Equal(at(s, "2030-01-10", 11, 0)) {
		t.Fatalf("first slot = %v, want 11:00", got)
	}
}

func TestSlotsBackToBackIsAllowed(t *testing.T) {
	s := newTestSchedule(t)
	day, _ := s.ParseDate("2030-01-10")
	now := at(s, "2030-01-01", 0, 0)
	busy := []Range{{Start: at(s, "2030-01-10", 9, 0), End: at(s, "2030-01-10", 10, 0)}}

	got := s.Slots(day, 30*time.Minute, busy, now)
	if len(got) == 0 || !got[0].Equal(at(s, "2030-01-10", 10, 0)) {
		t.Fatalf("a booking may start exactly when the previous one ends, got first=%v", got)
	}
}

func TestValidStart(t *testing.T) {
	s := newTestSchedule(t)
	hour := 60 * time.Minute

	cases := []struct {
		name  string
		start time.Time
		dur   time.Duration
		want  bool
	}{
		{"opening time", at(s, "2030-01-10", 9, 0), hour, true},
		{"before opening", at(s, "2030-01-10", 8, 30), hour, false},
		{"last fitting slot", at(s, "2030-01-10", 17, 0), hour, true},
		{"runs past closing", at(s, "2030-01-10", 17, 30), hour, false},
		{"off the grid", at(s, "2030-01-10", 10, 10), hour, false},
		{"2h service too late", at(s, "2030-01-10", 16, 30), 2 * hour, false},
		{"2h service fits", at(s, "2030-01-10", 16, 0), 2 * hour, true},
	}
	for _, c := range cases {
		if got := s.ValidStart(c.start, c.dur); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestValidStartUsesShopTimezone(t *testing.T) {
	s := newTestSchedule(t)
	// 10:00 in Bangkok (+07) is 03:00 UTC. The same instant expressed in UTC must still be valid.
	utc := at(s, "2030-01-10", 10, 0).UTC()
	if !s.ValidStart(utc, time.Hour) {
		t.Errorf("10:00 Bangkok given as %v should be valid", utc)
	}
}

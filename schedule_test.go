package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func at(hhmm string) time.Time {
	t := mustParseTimeOfDay(hhmm)
	return t.on(time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local))
}

func TestParseTimeOfDay(t *testing.T) {
	for _, s := range []string{"7:00", "07:00"} {
		got, err := parseTimeOfDay(s)
		if err != nil || got.String() != "07:00" {
			t.Errorf("parseTimeOfDay(%q) = %v, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "7", "24:00", "12:60", "ab:cd"} {
		if _, err := parseTimeOfDay(s); err == nil {
			t.Errorf("parseTimeOfDay(%q) expected error", s)
		}
	}
}

func TestMaxVolume(t *testing.T) {
	s := defaultSchedule()
	cases := map[string]int{
		"06:59": 20,
		"07:00": 40,
		"12:00": 40,
		"18:29": 40,
		"18:30": 20,
		"23:00": 20,
		"03:00": 20,
	}
	for hhmm, want := range cases {
		if got := s.MaxVolume(at(hhmm)); got != want {
			t.Errorf("MaxVolume(%s) = %d, want %d", hhmm, got, want)
		}
	}
}

func TestPresetVolume(t *testing.T) {
	s := defaultSchedule()
	s.DayPresetVolume = 50
	cases := []struct {
		hhmm        string
		limit, want int
	}{
		{"19:00", 20, 16}, // 80% of the night max
		{"19:00", 15, 12},
		{"12:00", 40, 20}, // 50% of the day max
		{"12:00", 30, 15}, // a lower profile limit
	}
	for _, c := range cases {
		if got := s.PresetVolume(at(c.hhmm), c.limit); got != c.want {
			t.Errorf("PresetVolume(%s, %d) = %d, want %d", c.hhmm, c.limit, got, c.want)
		}
	}
	s.NightPresetVolume = 0
	if got := s.PresetVolume(at("19:00"), 20); got != 0 {
		t.Errorf("disabled: got %d", got)
	}
}

func TestSleepTimerAt(t *testing.T) {
	s := defaultSchedule()
	cases := map[string]time.Duration{
		"18:29": 0,
		"18:30": 30 * time.Minute,
		"23:59": 30 * time.Minute,
		"00:00": 30 * time.Minute,
		"03:59": 30 * time.Minute,
		"04:00": 0,
		"12:00": 0,
	}
	for hhmm, want := range cases {
		if got := s.SleepTimerAt(at(hhmm)); got != want {
			t.Errorf("SleepTimerAt(%s) = %s, want %s", hhmm, got, want)
		}
	}
	s.SleepTimer = 0
	if got := s.SleepTimerAt(at("20:00")); got != 0 {
		t.Errorf("disabled sleep timer returned %s", got)
	}
}

func TestFormatSleepTimer(t *testing.T) {
	if got := formatSleepTimer(30 * time.Minute); got != "00:30:00" {
		t.Errorf("got %s", got)
	}
	if got := formatSleepTimer(90*time.Minute + 5*time.Second); got != "01:30:05" {
		t.Errorf("got %s", got)
	}
}

func TestCrossed(t *testing.T) {
	cutoff := mustParseTimeOfDay("19:45")
	if !crossed(at("19:44").Add(50*time.Second), at("19:45").Add(5*time.Second), cutoff) {
		t.Error("expected cutoff to be crossed")
	}
	if crossed(at("19:45").Add(5*time.Second), at("19:45").Add(20*time.Second), cutoff) {
		t.Error("cutoff should fire only once")
	}
	midnight := mustParseTimeOfDay("00:00")
	if !crossed(at("00:00").Add(-10*time.Second), at("00:00").Add(5*time.Second), midnight) {
		t.Error("expected midnight to be crossed")
	}
}

func TestTrackFilename(t *testing.T) {
	got := trackFilename("http://10.0.0.1:8080/music/presets/g/03-Love%2C%20Dont%20Worry.mp3")
	if got != "03-Love, Dont Worry.mp3" {
		t.Errorf("got %q", got)
	}
}

func TestPresetHandlerLetters(t *testing.T) {
	for _, preset := range []string{"g", "G"} {
		req := httptest.NewRequest("GET", "/sonos/preset/"+preset, nil)
		rr := httptest.NewRecorder()
		presetHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("preset %s: got status %d: %s", preset, rr.Code, rr.Body.String())
		}
	}
	req := httptest.NewRequest("GET", "/sonos/preset/a.b", nil)
	rr := httptest.NewRecorder()
	presetHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid preset: got status %d", rr.Code)
	}
}

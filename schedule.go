package main

import (
	"fmt"
	"log"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ianr0bkny/go-sonos"
	"github.com/ianr0bkny/go-sonos/ssdp"
	"github.com/ianr0bkny/go-sonos/upnp"
)

// timeOfDay is a wall clock time expressed as minutes since midnight in the
// server's local time zone.
type timeOfDay int

// parseTimeOfDay parses a 24-hour "HH:MM" string such as "07:00" or "18:30".
func parseTimeOfDay(s string) (timeOfDay, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time of day %q: expected HH:MM", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour in time of day %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in time of day %q", s)
	}
	return timeOfDay(h*60 + m), nil
}

func mustParseTimeOfDay(s string) timeOfDay {
	t, err := parseTimeOfDay(s)
	if err != nil {
		panic(err)
	}
	return t
}

func (t timeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60)
}

func timeOfDayOf(now time.Time) timeOfDay {
	return timeOfDay(now.Hour()*60 + now.Minute())
}

// on returns the instant t occurs on the calendar day of day.
func (t timeOfDay) on(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), int(t)/60, int(t)%60, 0, 0, day.Location())
}

// inWindow reports whether now falls within [start, end). The window wraps
// past midnight when end is not after start, e.g. 18:30 to 04:00.
func inWindow(now time.Time, start, end timeOfDay) bool {
	t := timeOfDayOf(now)
	if start < end {
		return t >= start && t < end
	}
	return t >= start || t < end
}

// crossed reports whether t occurred in the interval (prev, now].
func crossed(prev, now time.Time, t timeOfDay) bool {
	for _, day := range []time.Time{now, now.AddDate(0, 0, -1)} {
		occ := t.on(day)
		if occ.After(prev) && !occ.After(now) {
			return true
		}
	}
	return false
}

// Schedule holds the time of day behavior configured by command line flags.
type Schedule struct {
	DayStart       timeOfDay
	DayMaxVolume   int
	NightStart     timeOfDay
	NightMaxVolume int

	SleepTimerStart timeOfDay
	SleepTimerEnd   timeOfDay
	SleepTimer      time.Duration

	Cutoff        timeOfDay
	CutoffEnabled bool
}

func defaultSchedule() Schedule {
	return Schedule{
		DayStart:        mustParseTimeOfDay("07:00"),
		DayMaxVolume:    40,
		NightStart:      mustParseTimeOfDay("18:30"),
		NightMaxVolume:  15,
		SleepTimerStart: mustParseTimeOfDay("18:30"),
		SleepTimerEnd:   mustParseTimeOfDay("04:00"),
		SleepTimer:      30 * time.Minute,
		Cutoff:          mustParseTimeOfDay("19:45"),
		CutoffEnabled:   true,
	}
}

// schedule is the active schedule, configured from flags in main.
var schedule = defaultSchedule()

// MaxVolume returns the maximum volume allowed at now.
func (s Schedule) MaxVolume(now time.Time) int {
	if inWindow(now, s.DayStart, s.NightStart) {
		return s.DayMaxVolume
	}
	return s.NightMaxVolume
}

// SleepTimerAt returns the sleep timer to set when a preset starts at now, or
// zero if no sleep timer applies.
func (s Schedule) SleepTimerAt(now time.Time) time.Duration {
	if s.SleepTimer <= 0 || !inWindow(now, s.SleepTimerStart, s.SleepTimerEnd) {
		return 0
	}
	return s.SleepTimer
}

// formatSleepTimer formats d as the HH:MM:SS duration Sonos expects.
func formatSleepTimer(d time.Duration) string {
	secs := int(d.Round(time.Second) / time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", secs/3600, (secs/60)%60, secs%60)
}

// connectSpeaker looks up a cached speaker by name and connects to the given
// Sonos services.
func connectSpeaker(name string, services int) (*sonos.Sonos, Speaker, error) {
	speaker, exists := speakerCache[name]
	if !exists {
		return nil, speaker, fmt.Errorf("speaker '%s' not found", name)
	}
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		return nil, speaker, fmt.Errorf("failed to describe %s: %w", speaker.Name, err)
	}
	s := sonos.MakeSonos(svcMap, nil, services)
	if s == nil {
		return nil, speaker, fmt.Errorf("failed to connect to %s", speaker.Name)
	}
	return s, speaker, nil
}

// enforceMaxVolume lowers the volume to the current limit if it is above it.
func enforceMaxVolume(s *sonos.Sonos, speakerName string, now time.Time) error {
	limit := schedule.MaxVolume(now)
	current, err := s.GetVolume(0, "Master")
	if err != nil {
		return fmt.Errorf("failed to get volume: %w", err)
	}
	if int(current) <= limit {
		return nil
	}
	if err := s.SetVolume(0, "Master", uint16(limit)); err != nil {
		return fmt.Errorf("failed to set volume: %w", err)
	}
	log.Printf("Lowered volume on %s from %d to max %d", speakerName, current, limit)
	return nil
}

// ensureStopped pauses the speaker if it is playing, falling back to stop for
// sources which do not support pause.
func ensureStopped(s *sonos.Sonos, speakerName string) error {
	info, err := s.GetTransportInfo(0)
	if err != nil {
		return fmt.Errorf("failed to get transport info: %w", err)
	}
	state := info.CurrentTransportState
	if state != "PLAYING" && state != "TRANSITIONING" {
		log.Printf("Cutoff: %s is already %s", speakerName, state)
		return nil
	}
	if err := s.Pause(0); err != nil {
		log.Printf("Cutoff: pause failed on %s, trying stop: %v", speakerName, err)
		if err := s.Stop(0); err != nil {
			return fmt.Errorf("failed to stop playback: %w", err)
		}
	}
	log.Printf("Cutoff: stopped playback on %s", speakerName)
	return nil
}

// trackFilename returns the decoded file name of the track at uri.
func trackFilename(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return path.Base(uri)
}

// currentTrackFilename returns the file name of the track currently loaded on
// the speaker.
func currentTrackFilename(s *sonos.Sonos) (string, error) {
	info, err := s.GetPositionInfo(0)
	if err != nil {
		return "", err
	}
	return trackFilename(info.TrackURI), nil
}

// scheduledAction runs at a time of day, retrying until it succeeds or the
// retry window passes.
type scheduledAction struct {
	name         string
	at           timeOfDay
	run          func(now time.Time) error
	pendingSince time.Time
}

const (
	scheduleTick        = 15 * time.Second
	scheduleRetryWindow = 10 * time.Minute
)

// runScheduler performs time of day actions against the default speaker.
func runScheduler() {
	actions := []*scheduledAction{
		{
			name: "night volume limit",
			at:   schedule.NightStart,
			run: func(now time.Time) error {
				s, speaker, err := connectSpeaker(defaultSpeaker, sonos.SVC_RENDERING_CONTROL)
				if err != nil {
					return err
				}
				return enforceMaxVolume(s, speaker.Name, now)
			},
		},
	}
	if schedule.CutoffEnabled {
		actions = append(actions, &scheduledAction{
			name: "playback cutoff",
			at:   schedule.Cutoff,
			run: func(now time.Time) error {
				s, speaker, err := connectSpeaker(defaultSpeaker, sonos.SVC_AV_TRANSPORT)
				if err != nil {
					return err
				}
				return ensureStopped(s, speaker.Name)
			},
		})
	}

	prev := time.Now()
	ticker := time.NewTicker(scheduleTick)
	defer ticker.Stop()
	for now := range ticker.C {
		for _, a := range actions {
			if crossed(prev, now, a.at) {
				log.Printf("Schedule: %s at %s", a.name, a.at)
				a.pendingSince = now
			}
			if a.pendingSince.IsZero() {
				continue
			}
			if err := a.run(now); err != nil {
				log.Printf("Schedule: %s failed: %v", a.name, err)
				if now.Sub(a.pendingSince) > scheduleRetryWindow {
					log.Printf("Schedule: giving up on %s", a.name)
					a.pendingSince = time.Time{}
				}
				continue
			}
			a.pendingSince = time.Time{}
		}
		prev = now
	}
}

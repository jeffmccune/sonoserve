# sonoserve

Simple server for my 5 year old to control his Sonos Play:1 speaker using an
M5Stack CardPuter v1.1 esp32s3 device.

As a kid, I'd like to restart my favorite playlist so that I can listen to my favorite music.

As a kid, I'd like to play my favorite tracks.

The plan to implement these user stories is to put most of the logic in a Go executable and use the cardputer as a simple hotkey controller.  Ideally he will be able to turn it on, wait for a green light, then push one button to do the thing he wants to do.

## Time of day behavior

sonoserve limits volume and stops playback in the evening.  All times are
24-hour `HH:MM` in the server's local time zone.

| Flag | Default | Behavior |
|------|---------|----------|
| `-day-start` | `07:00` | Max volume becomes `-day-max-volume` |
| `-day-max-volume` | `40` | Max volume during the day |
| `-night-start` | `18:30` | Max volume becomes `-night-max-volume`; volume above it is lowered |
| `-night-max-volume` | `15` | Max volume at night |
| `-sleep-timer-start` | `18:30` | Presets started in this window set the Sonos sleep timer |
| `-sleep-timer-end` | `04:00` | End of the sleep timer window |
| `-sleep-timer` | `30m` | Sleep timer duration (`0` disables) |
| `-cutoff` | `19:45` | Playback on the default speaker is stopped at this time (empty disables) |

The max volume is enforced when a preset starts, on volume up, and at
`-night-start`.

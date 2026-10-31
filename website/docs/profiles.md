---
sidebar_position: 6
---

# Profiles and Device Settings

Every CardPuter runs the same firmware. A device stores only its settings, and
everything else comes from the server, so behavior changes by deploying a new
server rather than reprogramming each device.

| Profile | Person | Speaker |
|---|---|---|
| 1 | Clark | Kids Room |
| 2 | Rylo | Rylos Room |
| 3 | Jeff | Kids Room |

## Profiles on the server

Each profile is a folder in `profiles/`, embedded in the server executable:

```
profiles/1/profile.yaml   settings
profiles/1/avatar.jpg     default profile image
```

`profile.yaml` fields (see `profiles/1/profile.yaml` for a commented example):

| Field | Meaning |
|---|---|
| `name` | Shown on the ready screen. Required. |
| `speaker` | Sonos speaker the profile controls. Empty uses `-default-speaker`. |
| `image` | A file in the profile folder, or an `http(s)` URL such as `https://github.com/<user>.png`. Default `avatar.jpg`. |
| `color` | `#RRGGBB` accent color of the name. |
| `default_volume` | Volume a preset starts at, never above the limit. `0` uses `-day-preset-volume` or `-night-preset-volume` percent of the limit. |
| `max_volume` | Volume limit. The lower of this and the time of day schedule applies. `0` for only the schedule. |
| `help` | Lines shown on the ready screen. |
| `keys` | Key actions overriding the defaults. |

Unknown fields and invalid values stop the server from starting, and
`go test` checks the embedded profiles.

### Keys

The CardPuter sends every key press to `POST /profiles/{id}/key` and the
server decides what it does. The defaults match the original firmware: `0-9`
and `a-z` play the preset of the same name, `[` pause, `]` play, `,` previous,
`/` next, `;` volume up, `.` volume down. Override them per profile:

```yaml
keys:
  "[": play-pause
  m: mute
  z: none          # disable a key
  "!": preset:9
```

Actions: `preset:<name>`, `play`, `pause`, `play-pause`, `next`, `previous`,
`volume-up`, `volume-down`, `mute`, `none`. Keys with no action return
`204 No Content` and the CardPuter ignores them.

### Profile image

The server fetches the image, scales it to 96x96, and re-encodes it as a
baseline JPEG, so the CardPuter only ever downloads a small JPEG over plain
HTTP. Remote images (PNG, JPEG, or GIF) are cached for an hour. If a remote
image cannot be fetched, the server falls back to `avatar.jpg` in the profile
folder and tries again a minute later.

### Endpoints

| Request | Response |
|---|---|
| `GET /profiles/` | `[{"id":"1","name":"Clark"},...]` |
| `GET /profiles/{id}` | The profile as JSON, including `avatar_url`, `avatar_etag`, `color_rgb565`, `help`, and the effective `keys` |
| `GET /profiles/{id}/profile.yaml` | The settings file |
| `GET /profiles/{id}/avatar` | The image as a JPEG, with an `ETag` |
| `POST /profiles/{id}/key` | Performs the key's action and returns that action's response |

Key request body, where the optional fields are the device settings:

```json
{"key": "a", "speaker": "Kids Room", "default_volume": 15, "max_volume": 30}
```

The speaker is the device setting, else the profile speaker, else
`-default-speaker`. Volumes are the device setting when not 0, else the
profile. The bedtime volume limit and playback cutoff apply to the default
speaker, every profile speaker, and every speaker a device has controlled
since the server started.

## Device settings

Hold **Fn+S** while switching the CardPuter on, and keep holding until the
Settings screen appears (about a second after the screen lights up). A device
with no WiFi network saved opens settings automatically.

In settings, `;` and `.` move up and down, **Enter** chooses, and **Fn+`**
(Esc) goes back. Changes take effect with **Save and restart**.

| Setting | Meaning |
|---|---|
| WiFi 1-3 | Up to three networks, each with its own server URL (default `http://tools:8080`). At startup the device joins the first saved network in range. |
| Profile | Profile number, chosen from the server's list. |
| Speaker | Overrides the profile speaker, chosen from the discovered speakers. |
| Start volume | Overrides the profile `default_volume`. |
| Volume limit | Overrides the profile `max_volume`. |
| Erase all settings | Clears settings and cached images. |

On first start the firmware imports the JaM Family, Sound House, and custom
network passwords saved by the previous firmware.

At startup the device fetches its profile and caches it, with the image, in
flash, so the ready screen still shows when the server is down. The profile is
fetched again when the screen wakes if it is more than 10 minutes old.

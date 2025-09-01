# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

M5Stack CardPuter v1.1 (ESP32S3) firmware for controlling a Sonos speaker via HTTP REST API. The CardPuter acts as a wireless hotkey controller with visual feedback on a small LCD display.

## Development Environment

### Arduino IDE Setup
- Use Arduino IDE 2.3.6 or later
- Install M5Stack CardPuter board support following [M5Stack Arduino Setup Guide](https://docs.m5stack.com/en/arduino/arduino_ide)
- Board: M5Stack CardPuter v1.1 (ESP32S3)

### Key Libraries
- M5Cardputer: Display and keyboard interface
- WiFi: Network connectivity  
- HTTPClient: REST API communication
- Preferences: Persistent storage for WiFi credentials

## Architecture

### Network Configuration
The firmware supports multiple WiFi networks with priority-based connection:
1. "JaM Family" network (primary)
2. "Sound House" network (fallback with hardcoded server IP: 192.168.4.88)
3. Custom network (user-configurable)

Server endpoints:
- Default: `http://tools:8080/sonos/`
- Sound House: `http://192.168.4.88:8080/sonos/`

### Control Interface
Keyboard mappings:
- **0-9**: Play preset playlists
- **P**: Play/Pause toggle
- **M**: Mute toggle
- **,**: Previous track
- **/**: Next track
- **;**: Volume up
- **.**: Volume down

### Features
- Auto-connect to known networks with saved credentials
- 30-second screen timeout for battery conservation
- Battery level indicator with color coding
- Visual feedback for all commands (200 OK in white, errors in red)
- Password input with multiple confirm key options (Enter, Space, backtick, fn+M)

## Code Upload Process
1. Connect CardPuter via USB cable
2. Open controller.ino in Arduino IDE
3. Select correct board and port
4. Upload sketch

## API Communication
All requests are POST with JSON body:
```json
{"speaker": "Kids Room"}
```

Endpoints:
- `/sonos/preset/{1-9}` - Play preset playlist
- `/sonos/play` - Start playback
- `/sonos/pause` - Pause playback
- `/sonos/play-pause` - Toggle play/pause
- `/sonos/mute` - Toggle mute
- `/sonos/previous` - Previous track
- `/sonos/next` - Next track
- `/sonos/volume-up` - Increase volume
- `/sonos/volume-down` - Decrease volume
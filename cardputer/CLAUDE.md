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

Every device runs identical firmware. See `website/docs/profiles.md`.

- Device settings (NVS namespace `sonoserve`): up to 3 WiFi networks each with
  a server URL, profile number, optional speaker, start volume, and volume
  limit. Hold **Fn+S** at power on to open settings.
- The profile (name, image, color, speaker, volumes, help text) is fetched
  from `GET /profiles/{id}` at startup and cached in LittleFS.
- Every key press is sent to `POST /profiles/{id}/key`; the server decides
  what the key does, so behavior changes by deploying the server.
- 30-second screen timeout, battery indicator, artwork cached by ETag.

## Code Upload Process
1. Connect CardPuter via USB cable
2. Open controller.ino in Arduino IDE
3. Select correct board and port
4. Upload sketch

## API Communication
Key presses are POSTed with JSON body:
```json
{"key": "a", "speaker": "Kids Room", "default_volume": 15, "max_volume": 30}
```
`speaker` and the volumes are only sent when set in device settings. The
response is a track response (JSON with `filename`), a text status, or 204 if
the key does nothing.

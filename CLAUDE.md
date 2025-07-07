# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sonoserve is a simple server that allows a 5-year-old to control a Sonos Play:1 speaker using an M5Stack CardPuter v1.1 ESP32S3 device. The architecture consists of:
- Go HTTP server providing REST API endpoints for Sonos control
- ESP32 CardPuter client acting as a simple hotkey controller with visual feedback

## Development Commands

### Go Server
```bash
# Run the server (listens on port 8080)
make dev

# Build executable
go build

# Test endpoints
curl -s localhost:8080/health
curl -X POST localhost:8080/sonos/play
curl -X POST localhost:8080/sonos/pause
curl -X POST localhost:8080/sonos/restart-playlist
```

### ESP32 CardPuter Development
```bash
# Set up ESP-IDF environment
source $HOME/esp/esp-idf/export.sh
```

The CardPuter firmware is maintained at: https://github.com/jeffmccune/cardputer

## Architecture

The system uses a client-server model where the CardPuter sends HTTP requests to the Go server, which will control the Sonos speaker. The server currently has stub implementations for:

- `/health` - Health check endpoint
- `/sonos/play` - Play control (POST)
- `/sonos/pause` - Pause control (POST)
- `/sonos/restart-playlist` - Restart playlist (POST)

## Development Workflow

1. Always update `website/docs/prompts.md` with conversation history after each turn
2. Commit changes after each development step
3. The server uses graceful shutdown - handle SIGINT/SIGTERM properly

## Current State

- Basic HTTP server structure implemented

## Reminders

- Remember to update ./website/docs/prompts.md after every turn in the conversation, especially after context is compacted
- Remember to commit prompts.md after every turn in the conversation

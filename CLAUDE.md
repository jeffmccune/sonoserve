# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sonoserve is a simple server that allows a 5-year-old to control a Sonos Play:1 speaker using an M5Stack CardPuter v1.1 ESP32S3 device. The architecture consists of:
- Go HTTP server providing REST API endpoints for Sonos control
- ESP32 CardPuter client acting as a simple hotkey controller with visual feedback

## Development Commands
### Go Server
```bash
# Run the server (listens on port 8080, the human runs this)
./dev-loop.sh

# Trigger a rebuild (the llm runs this
./dev-rebuild.sh

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

1. Always trigger a rebuild and restart after each development step using Bash(./dev-rebuild.sh)
2. Always update `website/docs/prompts.md` with conversation history after each turn
3. Always commit changes after each development step

## Current State

- Basic HTTP server structure implemented
- Discovery endpoint implemented
- Basic Web UI implemented for testing in controller.md (simulates the http requests the esp32s3 will make)
- No work on esp32s3 started

## Reminders

- Remember to update ./website/docs/prompts.md after every turn in the conversation, especially after context is compacted
- Remember to commit prompts.md after every turn in the conversation
- Remember notes get added into the ./website/docs/notes.md file
- Always remember to update website/docs/prompts.md and commit each turn in the session

## Server Testing Workflow

- After making changes:
  1. Trigger a server rebuild
  2. Read server.status until it contains the string "ready"
  3. Perform tests against the running server

## Development Scripts

- Use `./dev-loop.sh` to run the development server with automatic rebuild capability
- Use `./dev-rebuild.sh` to trigger a server rebuild

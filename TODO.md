# TODO

This document tracks technical debt and future improvements for the sonoserve project.

## Build System

- [ ] Fix Makefile warnings: "warning: overriding commands for target 'build'" - appears to be a false positive but should investigate
- [ ] Add proper versioning system with semantic version tags
- [ ] Create GitHub Actions workflow for CI/CD
- [ ] Add Docker support with multi-stage builds
- [ ] Add proper release process with goreleaser

## Sonos Integration

- [ ] Replace mock Sonos discovery with actual SSDP discovery
  - Current implementation returns hardcoded test speaker
  - Need to properly implement network discovery
  - Consider using a maintained Sonos library
- [ ] Implement actual Sonos control commands (play, pause, restart)
- [ ] Add volume control endpoints
- [ ] Add playlist management
- [ ] Add speaker grouping support
- [ ] Implement proper error handling for Sonos API failures

## Security & Configuration

- [ ] Add authentication for API endpoints
- [ ] Implement HTTPS support with TLS certificates
- [ ] Add CORS configuration for web UI
- [ ] Create configuration file support (YAML/TOML)
- [ ] Add environment variable configuration
- [ ] Implement rate limiting for API endpoints

## CardPuter Integration

- [ ] Create ESP32 firmware for M5Stack CardPuter
- [ ] Implement WiFi configuration on device
- [ ] Add physical button mappings
- [ ] Implement LED status indicators
- [ ] Add device pairing/discovery protocol
- [ ] Create OTA update mechanism

## Web UI Improvements

- [ ] Convert controller page to proper React component
- [ ] Add real-time status updates (WebSocket/SSE)
- [ ] Improve mobile responsiveness
- [ ] Add dark mode support
- [ ] Create settings page for configuration
- [ ] Add visualization of currently playing track

## Testing & Quality

- [ ] Add integration tests for Sonos API
- [ ] Add benchmarks for embedded file serving
- [ ] Increase test coverage to >80%
- [ ] Add E2E tests for web UI
- [ ] Set up proper logging framework
- [ ] Add metrics and monitoring (Prometheus/OpenTelemetry)

## Documentation

- [ ] Add API documentation (OpenAPI/Swagger)
- [ ] Create user guide for parents setting up the system
- [ ] Add troubleshooting guide
- [ ] Document CardPuter button functions
- [ ] Create architecture decision records (ADRs)

## Performance

- [ ] Optimize embedded file serving with caching headers
- [ ] Add gzip compression for web assets
- [ ] Implement connection pooling for Sonos communication
- [ ] Profile and optimize memory usage

## Features

- [ ] Add support for multiple Sonos zones
- [ ] Implement preset/favorite management
- [ ] Add scheduler for automatic playback
- [ ] Create parent control interface
- [ ] Add usage statistics and parental reports
- [ ] Implement voice feedback on CardPuter

## Code Quality

- [ ] Add pre-commit hooks for formatting and linting
- [ ] Set up dependency scanning
- [ ] Add security scanning (gosec)
- [ ] Implement structured logging
- [ ] Add context propagation for tracing

## Deployment

- [ ] Create systemd service file
- [ ] Add health check endpoint with dependencies
- [ ] Create Kubernetes manifests
- [ ] Add Helm chart
- [ ] Set up automated backups for configuration

---

*Last updated: 2025-07-06*
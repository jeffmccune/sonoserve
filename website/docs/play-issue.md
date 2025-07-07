# Plan to fix Play endpoint

## Issue Analysis

The play endpoint is failing with error code 714 when trying to set the queue URI. Based on the stderr log analysis:

1. **Speaker Discovery**: Working correctly - "Kids Room" speaker found at 192.168.4.129
2. **Connection**: Successfully connects to the Sonos device and loads XML descriptions
3. **Queue Operations**: Successfully clears queue and adds track to queue
4. **Failure Point**: "Failed to set queue URI: 714" when trying to start playback

## Root Cause

Error 714 in UPnP/Sonos context typically indicates:
- **714 "Illegal MIME-Type"**: The content type of the media file is not supported or incorrectly specified
- The MP3 file URL `http://192.168.4.134:8080/music/sample.mp3` may be serving with wrong Content-Type header

## Troubleshooting Steps

### 1. Verify MP3 File Serving
- Check if the embedded MP3 file is being served correctly
- Test: `curl -I http://192.168.4.134:8080/music/sample.mp3`
- Expected: `Content-Type: audio/mpeg` or `audio/mp3`

### 2. Check Server IP Address
- The log shows server IP as 192.168.4.134 but requests come from different IP
- Verify the server is accessible from the Sonos device at that IP

### 3. Test Direct MP3 Access
- Verify the Sonos device can actually reach the MP3 URL
- Test from another device on the same network

### 4. Content-Type Header Fix
- Ensure the Go server's file serving handler sets correct MIME type
- Look for the `/music/` endpoint handler in main.go
- Verify it sets `Content-Type: audio/mpeg` header

### 5. Alternative Approaches
- Try using local file path instead of HTTP URL if supported
- Check if Sonos requires specific HTTP headers (User-Agent, etc.)
- Verify MP3 file format compatibility (bitrate, encoding)

## Next Actions

1. First verify the MP3 endpoint is serving correct Content-Type
2. Fix MIME type if incorrect
3. Test the play endpoint again
4. If still failing, investigate network connectivity between server and Sonos device
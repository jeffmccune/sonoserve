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

## Queue Endpoint Test Results

**Test Command:**
```bash
curl -X POST localhost:8080/sonos/queue -H "Content-Type: application/json" -d '{"speaker": "Kids Room"}'
```

**Result:**
```json
{"queue_contents":[{}],"queue_length":1,"speaker":"Kids Room"}
```

**Analysis:**
- Queue endpoint works correctly
- Shows 1 item in queue with empty metadata (`{}`)
- This suggests the track was added to the queue but metadata is not populated
- The queue contains the track from the previous failed play attempt

## Specific Failure Points in main.go

**Play Handler Function**: Lines 272-405

**Key Operation Points:**
1. **Queue Clearing**: Line 322 - `s.RemoveAllTracksFromQueue(0)` - ✅ Works
2. **Track Addition**: Line 356 - `s.AddURIToQueue(0, req)` - ✅ Works (confirmed by queue endpoint)
3. **Queue URI Setting**: Line 385 - `s.SetAVTransportURI(0, "Q:0", "")` - ❌ **FAILURE POINT**
4. **Playback Start**: Line 393 - `s.Play(0, "1")` - Not reached

**Exact Failure Location:**
- **File**: `/Users/jeff/esp/sonoserve/main.go`
- **Line**: 385
- **Function**: `playHandler`
- **Operation**: `s.SetAVTransportURI(0, "Q:0", "")`
- **Error**: "Failed to set queue URI: 714" (Illegal MIME-Type)

## Updated Root Cause Analysis

The issue is NOT with:
- Speaker discovery (✅ working)
- Queue clearing (✅ working)
- Track addition to queue (✅ working - confirmed by queue endpoint)

The issue IS with:
- **MIME type of the MP3 file** being served at `http://192.168.4.134:8080/music/sample.mp3`
- The Sonos device rejects the queue URI because it cannot accept the content type

## Next Actions

1. **Test MP3 endpoint directly:**
   ```bash
   curl -I http://192.168.4.134:8080/music/sample.mp3
   ```

2. **Check Content-Type header** - Should be `audio/mpeg` or `audio/mp3`

3. **Find and fix the music endpoint handler** in main.go

4. **Verify MP3 file format** - Ensure it's compatible with Sonos requirements
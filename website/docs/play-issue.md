# Plan to fix Play endpoint

## Issue Analysis (Updated)

The play endpoint is failing with error code 714 when trying to set the queue URI. After thorough investigation, the root cause has been identified.

### Current Status
1. **Speaker Discovery**: ✅ Working correctly - "Kids Room" speaker found at 192.168.4.129
2. **Connection**: ✅ Successfully connects to the Sonos device and loads XML descriptions  
3. **Queue Operations**: ✅ Successfully clears queue and adds track to queue
4. **Failure Point**: ❌ **SetAVTransportURI call fails with error 714**

## Troubleshooting Results

### 1. MP3 File Serving Test
**Command:** `curl -I http://192.168.4.134:8080/music/sample.mp3`

**Output:**
```
HTTP/1.1 200 OK
Accept-Ranges: bytes
Access-Control-Allow-Headers: Content-Type, Authorization
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Origin: *
Content-Length: 103070
Content-Type: audio/mpeg
Date: Mon, 07 Jul 2025 03:56:00 GMT
```

**Result:** ✅ **MP3 file is serving correctly with proper Content-Type: audio/mpeg**

### 2. Queue State Verification
**Command:** `curl -X POST localhost:8080/sonos/queue -H "Content-Type: application/json" -d '{"speaker": "Kids Room"}' | jq .`

**Output:**
```json
{
  "queue_items": [
    {
      "album": "",
      "album_art_uri": "/getaa?u=http%3a%2f%2f192.168.4.134%3a8080%2fmusic%2fsample.mp3&v=1",
      "class": "object.item",
      "creator": "",
      "id": "Q:0/1",
      "index": 0,
      "parent_id": "Q:0",
      "restricted": true,
      "title": "sample.mp3",
      "track_number": "",
      "uri": "http://192.168.4.134:8080/music/sample.mp3"
    }
  ],
  "queue_length": 1,
  "raw_contents": [{}],
  "speaker": "Kids Room"
}
```

**Result:** ✅ **Track is successfully added to queue with correct URI and metadata**

### 3. Play Endpoint Test
**Command:** `curl -X POST localhost:8080/sonos/play -H "Content-Type: application/json" -d '{"speaker": "Kids Room"}'`

**Output:** `Failed to set queue for playback`

**Stderr Log:**
```
2025/07/06 20:56:20 Play requested for speaker: Kids Room
2025/07/06 20:56:20 Loading http://192.168.4.129:1400/xml/device_description.xml
2025/07/06 20:56:20 Loading http://192.168.4.129:1400/xml/AVTransport1.xml
2025/07/06 20:56:20 Clearing current queue on Kids Room
2025/07/06 20:56:20 Adding track to queue: http://192.168.4.134:8080/music/sample.mp3
2025/07/06 20:56:20 Added track http://192.168.4.134:8080/music/sample.mp3 at position 1
2025/07/06 20:56:20 Added 1 tracks to queue, setting up playback from queue
2025/07/06 20:56:20 Failed to set queue URI: 714
```

**Result:** ❌ **Failure occurs at SetAVTransportURI call with error 714**

## Root Cause (Corrected)

**The issue is NOT with MIME types** - that was a red herring. The actual problem is:

### Incorrect SetAVTransportURI Usage

**Current problematic code (line 389 in main.go):**
```go
err = s.SetAVTransportURI(0, "Q:0", "")
```

**Problem:** `"Q:0"` is an object ID, not a playable URI. The SetAVTransportURI function expects an actual resource URI.

### Missing Required Service

**Current code uses only:**
```go
s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT)
```

**Problem:** Also needs `SVC_CONTENT_DIRECTORY` to retrieve queue metadata.

## Technical Solution

### Required Changes

#### 1. Update Service Configuration
```go
// Current (WRONG)
s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT)

// Correct (FIXED)
s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT|sonos.SVC_CONTENT_DIRECTORY)
```

#### 2. Replace SetAVTransportURI Call
```go
// Current (WRONG)
err = s.SetAVTransportURI(0, "Q:0", "")

// Correct (FIXED)
// Get queue metadata to obtain the correct playable URI
if data, err := s.GetMetadata(sonos.ObjectID_Queue_AVT_Instance_0); err != nil {
    log.Printf("Failed to get queue metadata: %v", err)
    http.Error(w, "Failed to get queue metadata", http.StatusInternalServerError)
    return
} else {
    // Use the actual resource URI from metadata
    if err := s.SetAVTransportURI(0, data[0].Res(), ""); err != nil {
        log.Printf("Failed to set queue URI: %v", err)
        http.Error(w, "Failed to set queue for playback", http.StatusInternalServerError)
        return
    }
}
```

## Implementation Plan

### Step 1: Update playHandler Service Configuration
- **File:** `/Users/jeff/esp/sonoserve/main.go`
- **Location:** Around line 319 (in playHandler function)
- **Change:** Add `SVC_CONTENT_DIRECTORY` to the service flags

### Step 2: Replace SetAVTransportURI Logic
- **File:** `/Users/jeff/esp/sonoserve/main.go`
- **Location:** Around line 389 (current SetAVTransportURI call)
- **Change:** Use GetMetadata approach as shown above

### Step 3: Test and Verify
1. Rebuild server with changes
2. Test play endpoint
3. Verify playback starts successfully
4. Check stderr log for any remaining errors

## Expected Outcome

After implementing these changes:
1. GetMetadata will return the proper queue URI
2. SetAVTransportURI will succeed with the correct URI
3. Play() call will start playback from the queue
4. Music should start playing on the Sonos speaker

## Sequence Flow (Corrected)

```
1. Clear queue                    ✅ Working
2. Add tracks to queue           ✅ Working  
3. Get queue metadata            🔧 NEW STEP
4. Set AV Transport URI          🔧 FIXED METHOD
5. Start playback               🎵 Should work
```

This approach follows the correct go-sonos library patterns and should resolve the error 714 issue.
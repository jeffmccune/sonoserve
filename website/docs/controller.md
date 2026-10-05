---
sidebar_position: 3
---

# Speaker Controller

Control your Sonos speakers from this interface.

## Server Configuration

<div style={{marginBottom: '20px'}}>
  <label htmlFor="serverInput" style={{marginRight: '10px'}}>Server (host:port):</label>
  <input 
    id="serverInput" 
    type="text" 
    defaultValue="localhost:8080"
    style={{
      padding: '5px 10px',
      fontSize: '16px',
      border: '1px solid #ccc',
      borderRadius: '4px',
      width: '200px'
    }}
  />
</div>

## Device Discovery

Get the current list of discovered speakers or refresh by discovering new ones:

<div style={{marginBottom: '10px'}}>
  <button 
    id="getSpeakersBtn"
    onClick={() => {
      const button = document.getElementById('getSpeakersBtn');
      const list = document.getElementById('speakersList');
      const serverInput = document.getElementById('serverInput');
      const server = serverInput.value || 'localhost:8080';
      
      button.disabled = true;
      button.textContent = 'Getting...';
      
      // Use relative URL if we're on the same host (development mode with proxy)
      const url = (window.location.host === server) 
        ? '/api/sonos/speakers' 
        : `http://${server}/api/sonos/speakers`;
      
      fetch(url, { method: 'GET' })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.json();
        })
        .then(speakers => {
          if (speakers.length === 0) {
            list.innerHTML = '<em>No speakers in cache. Click "Refresh Speakers" to discover speakers.</em>';
            document.getElementById('speakerSelection').innerHTML = '<em>No speakers available. Please refresh speakers first.</em>';
          } else {
            list.innerHTML = '<h4>Cached Speakers:</h4><ul>' + 
              speakers.map(speaker => `<li>${speaker.name} (${speaker.room}) - ${speaker.address}</li>`).join('') + 
              '</ul>';
            
            // Update speaker selection radio buttons
            const speakerSelection = document.getElementById('speakerSelection');
            speakerSelection.innerHTML = '<h4>Select Speaker:</h4>' + 
              speakers.map((speaker, index) => 
                `<div style="margin: 8px 0;">
                  <input type="radio" id="speaker${index}" name="speaker" value="${speaker.name}" ${index === 0 ? 'checked' : ''} />
                  <label for="speaker${index}" style="margin-left: 8px; cursor: pointer;">
                    ${speaker.name} (${speaker.room})
                  </label>
                </div>`
              ).join('');
          }
        })
        .catch(error => {
          list.innerHTML = `<em style={{color: 'red'}}>Error: ${error.message}</em>`;
        })
        .finally(() => {
          button.disabled = false;
          button.textContent = 'Get Speakers';
        });
    }}
    style={{marginRight: '10px'}}
  >
    Get Speakers
  </button>
  <button 
    id="discoverBtn"
    onClick={() => {
      const button = document.getElementById('discoverBtn');
      const list = document.getElementById('speakersList');
      const serverInput = document.getElementById('serverInput');
      const server = serverInput.value || 'localhost:8080';
      
      button.disabled = true;
      button.textContent = 'Refreshing...';
      list.innerHTML = '<em>Searching for speakers...</em>';
      
      // Use relative URL if we're on the same host (development mode with proxy)
      const url = (window.location.host === server) 
        ? '/api/sonos/discover' 
        : `http://${server}/api/sonos/discover`;
      
      fetch(url, { method: 'POST' })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.json();
        })
        .then(speakers => {
          if (speakers.length === 0) {
            list.innerHTML = '<em>No speakers found. Make sure your Sonos devices are on the same network.</em>';
            document.getElementById('speakerSelection').innerHTML = '<em>No speakers found.</em>';
          } else {
            list.innerHTML = '<h4>Found Speakers:</h4><ul>' + 
              speakers.map(speaker => `<li>${speaker.name} - ${speaker.ip}</li>`).join('') + 
              '</ul>';
            
            // Update speaker selection radio buttons
            const speakerSelection = document.getElementById('speakerSelection');
            speakerSelection.innerHTML = '<h4>Select Speaker:</h4>' + 
              speakers.map((speaker, index) => 
                `<div style="margin: 8px 0;">
                  <input type="radio" id="speaker${index}" name="speaker" value="${speaker.name}" ${index === 0 ? 'checked' : ''} />
                  <label for="speaker${index}" style="margin-left: 8px; cursor: pointer;">
                    ${speaker.name}
                  </label>
                </div>`
              ).join('');
          }
        })
        .catch(error => {
          list.innerHTML = `<em style={{color: 'red'}}>Error: ${error.message}</em>`;
        })
        .finally(() => {
          button.disabled = false;
          button.textContent = 'Refresh Speakers';
        });
    }}
  >
    Refresh Speakers
  </button>
</div>

<div id="speakersList" style={{marginTop: '20px', padding: '10px', border: '1px solid #ddd', minHeight: '100px'}}>
  <em>No speakers discovered yet. Click the button above to search.</em>
</div>

## Speaker Selection

Select which speaker to control:

<div id="speakerSelection" style={{marginTop: '20px', marginBottom: '20px', padding: '15px', border: '1px solid #ddd', borderRadius: '4px'}}>
  <em>Load speakers first using the "Get Speakers" button above to see available speakers.</em>
</div>

## Music Controls

<div style={{marginTop: '30px'}}>
  <button 
    onClick={() => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/play' 
        : `http://${server}/sonos/play`;
      
      const playButton = document.querySelector('button[data-action="play"]');
      playButton.disabled = true;
      playButton.textContent = '⏳ Starting...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Play result:', result);
          playButton.textContent = '✅ Play';
          setTimeout(() => {
            playButton.textContent = '▶️ Play';
          }, 2000);
        })
        .catch(error => {
          console.error('Play error:', error);
          playButton.textContent = '❌ Play';
          setTimeout(() => {
            playButton.textContent = '▶️ Play';
          }, 2000);
        })
        .finally(() => {
          playButton.disabled = false;
        });
    }} 
    style={{marginRight: '10px'}}
    data-action="play"
  >
    ▶️ Play
  </button>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const pauseButton = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/pause' 
        : `http://${server}/sonos/pause`;
      
      pauseButton.disabled = true;
      pauseButton.textContent = '⏳ Pausing...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Pause result:', result);
          pauseButton.textContent = '✅ Pause';
          setTimeout(() => {
            pauseButton.textContent = '⏸️ Pause';
          }, 2000);
        })
        .catch(error => {
          console.error('Pause error:', error);
          pauseButton.textContent = '❌ Pause';
          setTimeout(() => {
            pauseButton.textContent = '⏸️ Pause';
          }, 2000);
        })
        .finally(() => {
          pauseButton.disabled = false;
        });
    }} 
    style={{marginRight: '10px'}}
  >
    ⏸️ Pause
  </button>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const restartButton = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/restart-playlist' 
        : `http://${server}/sonos/restart-playlist`;
      
      restartButton.disabled = true;
      restartButton.textContent = '⏳ Restarting...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Restart result:', result);
          restartButton.textContent = '✅ Restart Playlist';
          setTimeout(() => {
            restartButton.textContent = '🔄 Restart Playlist';
          }, 2000);
        })
        .catch(error => {
          console.error('Restart playlist error:', error);
          restartButton.textContent = '❌ Restart Playlist';
          setTimeout(() => {
            restartButton.textContent = '🔄 Restart Playlist';
          }, 2000);
        })
        .finally(() => {
          restartButton.disabled = false;
        });
    }}
  >
    🔄 Restart Playlist
  </button>
</div>

## Additional Controls

<div style={{marginTop: '20px'}}>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const button = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/play-pause' 
        : `http://${server}/sonos/play-pause`;
      
      button.disabled = true;
      button.textContent = '⏳ Toggle...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Play/Pause result:', result);
          button.textContent = '✅ Play/Pause';
          setTimeout(() => {
            button.textContent = '⏯️ Play/Pause';
          }, 2000);
        })
        .catch(error => {
          console.error('Play/Pause error:', error);
          button.textContent = '❌ Play/Pause';
          setTimeout(() => {
            button.textContent = '⏯️ Play/Pause';
          }, 2000);
        })
        .finally(() => {
          button.disabled = false;
        });
    }} 
    style={{marginRight: '10px'}}
  >
    ⏯️ Play/Pause
  </button>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const button = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/next' 
        : `http://${server}/sonos/next`;
      
      button.disabled = true;
      button.textContent = '⏳ Next...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Next result:', result);
          button.textContent = '✅ Next';
          setTimeout(() => {
            button.textContent = '⏭️ Next';
          }, 2000);
        })
        .catch(error => {
          console.error('Next error:', error);
          button.textContent = '❌ Next';
          setTimeout(() => {
            button.textContent = '⏭️ Next';
          }, 2000);
        })
        .finally(() => {
          button.disabled = false;
        });
    }} 
    style={{marginRight: '10px'}}
  >
    ⏭️ Next
  </button>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const button = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/previous' 
        : `http://${server}/sonos/previous`;
      
      button.disabled = true;
      button.textContent = '⏳ Previous...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Previous result:', result);
          button.textContent = '✅ Previous';
          setTimeout(() => {
            button.textContent = '⏮️ Previous';
          }, 2000);
        })
        .catch(error => {
          console.error('Previous error:', error);
          button.textContent = '❌ Previous';
          setTimeout(() => {
            button.textContent = '⏮️ Previous';
          }, 2000);
        })
        .finally(() => {
          button.disabled = false;
        });
    }} 
    style={{marginRight: '10px'}}
  >
    ⏮️ Previous
  </button>
</div>

## Volume Controls

<div style={{marginTop: '20px'}}>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const button = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/volume-up' 
        : `http://${server}/sonos/volume-up`;
      
      button.disabled = true;
      button.textContent = '⏳ Volume Up...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Volume up result:', result);
          button.textContent = '✅ Volume Up';
          setTimeout(() => {
            button.textContent = '🔊 Volume Up';
          }, 2000);
        })
        .catch(error => {
          console.error('Volume up error:', error);
          button.textContent = '❌ Volume Up';
          setTimeout(() => {
            button.textContent = '🔊 Volume Up';
          }, 2000);
        })
        .finally(() => {
          button.disabled = false;
        });
    }} 
    style={{marginRight: '10px'}}
  >
    🔊 Volume Up
  </button>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const button = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/volume-down' 
        : `http://${server}/sonos/volume-down`;
      
      button.disabled = true;
      button.textContent = '⏳ Volume Down...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Volume down result:', result);
          button.textContent = '✅ Volume Down';
          setTimeout(() => {
            button.textContent = '🔉 Volume Down';
          }, 2000);
        })
        .catch(error => {
          console.error('Volume down error:', error);
          button.textContent = '❌ Volume Down';
          setTimeout(() => {
            button.textContent = '🔉 Volume Down';
          }, 2000);
        })
        .finally(() => {
          button.disabled = false;
        });
    }} 
    style={{marginRight: '10px'}}
  >
    🔉 Volume Down
  </button>
  <button 
    onClick={(e) => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const button = e.target;
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/mute' 
        : `http://${server}/sonos/mute`;
      
      button.disabled = true;
      button.textContent = '⏳ Mute...';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.text();
        })
        .then(result => {
          console.log('Mute result:', result);
          button.textContent = '✅ Mute';
          setTimeout(() => {
            button.textContent = '🔇 Mute';
          }, 2000);
        })
        .catch(error => {
          console.error('Mute error:', error);
          button.textContent = '❌ Mute';
          setTimeout(() => {
            button.textContent = '🔇 Mute';
          }, 2000);
        })
        .finally(() => {
          button.disabled = false;
        });
    }}
  >
    🔇 Mute
  </button>
</div>

## Presets

Quick access to preset playlists:

<div style={{marginTop: '20px', marginBottom: '20px'}}>
  {[0, 1, 2, 3, 4, 5, 6, 7, 8, 9].map(num => (
    <button
      key={num}
      onClick={() => {
        const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
        if (!selectedSpeaker) {
          alert('Please select a speaker first');
          return;
        }
        
        const server = document.getElementById('serverInput').value || 'localhost:8080';
        const url = (window.location.host === server) 
          ? `/sonos/preset/${num}` 
          : `http://${server}/sonos/preset/${num}`;
        
        const button = event.target;
        button.disabled = true;
        button.textContent = `⏳ ${num}`;
        
        fetch(url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            speaker: selectedSpeaker.value
          })
        })
          .then(response => {
            if (!response.ok) {
              throw new Error(`HTTP error! status: ${response.status}`);
            }
            return response.text();
          })
          .then(result => {
            console.log(`Preset ${num} result:`, result);
            button.textContent = '✅';
            setTimeout(() => {
              button.textContent = num;
            }, 2000);
          })
          .catch(error => {
            console.error(`Preset ${num} error:`, error);
            button.textContent = '❌';
            setTimeout(() => {
              button.textContent = num;
            }, 2000);
          })
          .finally(() => {
            button.disabled = false;
          });
      }}
      style={{
        margin: '5px',
        padding: '10px 20px',
        fontSize: '18px',
        fontWeight: 'bold',
        backgroundColor: '#f0f0f0',
        border: '1px solid #ccc',
        borderRadius: '4px',
        cursor: 'pointer',
        minWidth: '50px'
      }}
    >
      {num}
    </button>
  ))}
</div>

## View Preset Playlist

View the contents of any preset playlist before playing it:

<div style={{marginTop: '20px'}}>
  <label style={{marginRight: '10px'}}>Preset (0-9, a-z):</label>
  <input 
    id="presetInput" 
    type="text" 
    maxLength="1" 
    defaultValue="5"
    style={{
      padding: '5px 10px',
      fontSize: '16px',
      border: '1px solid #ccc',
      borderRadius: '4px',
      width: '60px',
      marginRight: '10px'
    }}
  />
  <button 
    onClick={() => {
      const presetNum = document.getElementById('presetInput').value.toLowerCase();
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? `/sonos/preset/${presetNum}` 
        : `http://${server}/sonos/preset/${presetNum}`;
      
      const button = event.target;
      const resultDiv = document.getElementById('presetResult');
      button.disabled = true;
      button.textContent = 'Loading Preset...';
      resultDiv.innerHTML = '<em>Fetching preset playlist...</em>';
      
      fetch(url, { method: 'GET' })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.json();
        })
        .then(data => {
          console.log('Preset data:', data);
          
          let content = `<h4>Preset ${data.preset} - ${data.playlist_count} tracks:</h4>`;
          
          if (data.playlist_count === 0) {
            content += '<p><em>This preset is empty (no tracks found)</em></p>';
          } else {
            content += '<ol style="padding-left: 20px;">';
            data.playlist_items.forEach(item => {
              content += `
                <li style="margin: 8px 0; padding: 8px; background-color: #f8f9fa; border-radius: 4px;">
                  <strong>${item.title}</strong><br/>
                  <small style="color: #6c757d;">File: ${item.filename}</small><br/>
                  <small style="color: #6c757d;">URL: <a href="${item.url}" target="_blank" style="color: #0969da;">${item.url}</a></small>
                </li>
              `;
            });
            content += '</ol>';
          }
          
          resultDiv.innerHTML = content;
        })
        .catch(error => {
          console.error('Preset error:', error);
          resultDiv.innerHTML = `<em style="color: red;">Error: ${error.message}</em>`;
        })
        .finally(() => {
          button.disabled = false;
          button.textContent = 'View Preset';
        });
    }}
    style={{
      padding: '10px 20px',
      fontSize: '16px',
      backgroundColor: '#28a745',
      color: 'white',
      border: 'none',
      borderRadius: '4px',
      cursor: 'pointer'
    }}
  >
    View Preset
  </button>
  
  <div id="presetResult" style={{
    marginTop: '20px',
    padding: '20px',
    border: '1px solid #ddd',
    borderRadius: '4px',
    minHeight: '100px',
    backgroundColor: '#fafbfc'
  }}>
    <em>Select a preset number and click "View Preset" to see its contents</em>
  </div>
</div>

## API Examples

Here are curl command examples for all the API endpoints:

### Health Check
```bash
curl -s localhost:8080/health
```

### Get Speakers (list cached speakers)
```bash
curl -s localhost:8080/api/sonos/speakers
```

### Discover Speakers (find new speakers)
```bash
curl -X POST localhost:8080/api/sonos/discover
```

### Track Responses

Play Preset, Next Track, Previous Track, Play, and Play/Pause (when it
resumes playback) respond with JSON describing the track now playing:

```json
{"preset":"7","speaker":"Kids Room","filename":"03-Come As You Are.mp3",
 "title":"Come As You Are (Remastered)","album":"Nevermind (30th Anniversary Super Deluxe)",
 "artist":"Nirvana","artwork_url":"/sonos/artwork/7/03-Come%20As%20You%20Are%20%28Remastered%29.jpg",
 "artwork_etag":"0aa012...","artwork_timeout_seconds":30}
```

- `title`, `album`, and `artist` come from the ID3 tag of the mp3 file. `title`
  falls back to the file name without `.mp3`. Empty `album` and `artist` are
  omitted.
- `artwork_url` is the track's own JPEG exported by `export-tags`, otherwise the
  preset's `artwork.jpg`. It and `artwork_etag` and `artwork_timeout_seconds`
  are omitted when there is neither.
- `preset` is empty when the track is not from a preset.

The CardPuter shows exactly what the response describes: the title, album, and
artwork of the track, replacing whatever it showed before.

### Play
```bash
# Queues every embedded mp3 file and responds with the track response.
curl -X POST localhost:8080/sonos/play \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Pause
```bash
curl -X POST localhost:8080/sonos/pause \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Restart Playlist
```bash
curl -X POST localhost:8080/sonos/restart-playlist \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Get Preset Playlist (View Contents)
```bash
# Replace {num} with a number 0-9 or a letter a-z
curl -s localhost:8080/sonos/preset/{num}

# Example for preset 5:
curl -s localhost:8080/sonos/preset/5
```

### Play Preset (0-9, a-z)
```bash
# Replace {num} with a number 0-9 or a letter a-z (upper case is lower cased).
# Responds with the track response of the first track.
curl -X POST localhost:8080/sonos/preset/{num} \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'

# Example for preset 5:
curl -X POST localhost:8080/sonos/preset/5 \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Get Preset Artwork
```bash
# Serves music/presets/{num}/artwork.jpg scaled to fit 135x135 as a baseline
# JPEG with an ETag. Responds 404 if the preset has no artwork.
curl -s -o artwork.jpg localhost:8080/sonos/artwork/9

# Serves a track's artwork file, e.g. music/presets/7/01-The Humbling River.jpg,
# the same way. Use the artwork_url of a track response.
curl -s -o track.jpg "localhost:8080/sonos/artwork/7/01-The%20Humbling%20River.jpg"
```

### Get Queue
```bash
curl -X POST localhost:8080/sonos/queue \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Play/Pause Toggle
```bash
# Responds with the track response when it resumes playback, otherwise text.
curl -X POST localhost:8080/sonos/play-pause \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Next Track
```bash
# Responds with the track response of the new current track.
curl -X POST localhost:8080/sonos/next \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Previous Track
```bash
# Responds with the track response of the new current track.
curl -X POST localhost:8080/sonos/previous \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Volume Up (5%)
```bash
curl -X POST localhost:8080/sonos/volume-up \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Volume Down (5%)
```bash
curl -X POST localhost:8080/sonos/volume-down \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

### Mute Toggle
```bash
curl -X POST localhost:8080/sonos/mute \
  -H "Content-Type: application/json" \
  -d '{"speaker": "Living Room"}'
```

## Get Speaker Queue

View the current queue on the selected speaker:

<div style={{marginTop: '20px'}}>
  <button 
    onClick={() => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/queue' 
        : `http://${server}/sonos/queue`;
      
      const button = event.target;
      const resultDiv = document.getElementById('queueResult');
      button.disabled = true;
      button.textContent = 'Loading Queue...';
      resultDiv.innerHTML = '<em>Fetching queue...</em>';
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          speaker: selectedSpeaker.value
        })
      })
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.json();
        })
        .then(data => {
          console.log('Queue data:', data);
          
          // Format the JSON with syntax highlighting
          const formattedJson = JSON.stringify(data, null, 2)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"([^"]+)":/g, '<span style="color: #0969da;">"$1"</span>:')
            .replace(/: "([^"]*)"/g, ': <span style="color: #0a3069;">"$1"</span>')
            .replace(/: (\d+)/g, ': <span style="color: #cf222e;">$1</span>')
            .replace(/: (true|false)/g, ': <span style="color: #8250df;">$1</span>')
            .replace(/: (null)/g, ': <span style="color: #6e7781;">$1</span>');
          
          resultDiv.innerHTML = `
            <h4>Queue for ${selectedSpeaker.value}:</h4>
            <pre style="
              background-color: #f6f8fa;
              border: 1px solid #d1d9e0;
              border-radius: 6px;
              padding: 16px;
              overflow-x: auto;
              font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
              font-size: 12px;
              line-height: 1.45;
            ">${formattedJson}</pre>
          `;
        })
        .catch(error => {
          console.error('Queue error:', error);
          resultDiv.innerHTML = `<em style="color: red;">Error: ${error.message}</em>`;
        })
        .finally(() => {
          button.disabled = false;
          button.textContent = 'Get Queue';
        });
    }}
    style={{
      padding: '10px 20px',
      fontSize: '16px',
      backgroundColor: '#0969da',
      color: 'white',
      border: 'none',
      borderRadius: '4px',
      cursor: 'pointer'
    }}
  >
    Get Queue
  </button>
  
  <div id="queueResult" style={{
    marginTop: '20px',
    padding: '20px',
    border: '1px solid #ddd',
    borderRadius: '4px',
    minHeight: '100px',
    backgroundColor: '#fafbfc'
  }}>
    <em>Click "Get Queue" to view the current queue</em>
  </div>
</div>
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
          } else {
            list.innerHTML = '<h4>Cached Speakers:</h4><ul>' + 
              speakers.map(speaker => `<li>${speaker.name} (${speaker.room}) - ${speaker.address}</li>`).join('') + 
              '</ul>';
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
          } else {
            list.innerHTML = '<h4>Found Speakers:</h4><ul>' + 
              speakers.map(speaker => `<li>${speaker.name} - ${speaker.ip}</li>`).join('') + 
              '</ul>';
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

## Music Controls

<div style={{marginTop: '30px'}}>
  <button 
    onClick={() => {
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/play' 
        : `http://${server}/sonos/play`;
      fetch(url, {method: 'POST'})
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
        })
        .catch(error => {
          console.error('Play error:', error);
          alert(`Failed to play: ${error.message}`);
        });
    }} 
    style={{marginRight: '10px'}}
  >
    ▶️ Play
  </button>
  <button 
    onClick={() => {
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/pause' 
        : `http://${server}/sonos/pause`;
      fetch(url, {method: 'POST'})
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
        })
        .catch(error => {
          console.error('Pause error:', error);
          alert(`Failed to pause: ${error.message}`);
        });
    }} 
    style={{marginRight: '10px'}}
  >
    ⏸️ Pause
  </button>
  <button 
    onClick={() => {
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/restart-playlist' 
        : `http://${server}/sonos/restart-playlist`;
      fetch(url, {method: 'POST'})
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
        })
        .catch(error => {
          console.error('Restart playlist error:', error);
          alert(`Failed to restart playlist: ${error.message}`);
        });
    }}
  >
    🔄 Restart Playlist
  </button>
</div>
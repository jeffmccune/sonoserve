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

Click the button below to discover Sonos speakers on your network:

<div>
  <button 
    id="discoverBtn"
    onClick={() => {
      const button = document.getElementById('discoverBtn');
      const list = document.getElementById('speakersList');
      const serverInput = document.getElementById('serverInput');
      const server = serverInput.value || 'localhost:8080';
      
      button.disabled = true;
      button.textContent = 'Discovering...';
      list.innerHTML = '<em>Searching for speakers...</em>';
      
      fetch(`http://${server}/api/sonos/discover`, { method: 'POST' })
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
          button.textContent = 'Discover Speakers';
        });
    }}
  >
    Discover Speakers
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
      fetch(`http://${server}/sonos/play`, {method: 'POST'})
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
      fetch(`http://${server}/sonos/pause`, {method: 'POST'})
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
      fetch(`http://${server}/sonos/restart-playlist`, {method: 'POST'})
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
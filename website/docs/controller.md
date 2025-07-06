---
sidebar_position: 3
---

# Speaker Controller

Control your Sonos speakers from this interface.

## Device Discovery

Click the button below to discover Sonos speakers on your network:

<div>
  <button 
    id="discoverBtn"
    onClick={() => {
      const button = document.getElementById('discoverBtn');
      const list = document.getElementById('speakersList');
      
      button.disabled = true;
      button.textContent = 'Discovering...';
      list.innerHTML = '<em>Searching for speakers...</em>';
      
      fetch('/api/sonos/discover', { method: 'POST' })
        .then(response => response.json())
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
    onClick={() => fetch('/api/sonos/play', {method: 'POST'})} 
    style={{marginRight: '10px'}}
  >
    ▶️ Play
  </button>
  <button 
    onClick={() => fetch('/api/sonos/pause', {method: 'POST'})} 
    style={{marginRight: '10px'}}
  >
    ⏸️ Pause
  </button>
  <button 
    onClick={() => fetch('/api/sonos/restart-playlist', {method: 'POST'})}
  >
    🔄 Restart Playlist
  </button>
</div>
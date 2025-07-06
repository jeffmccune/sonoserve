---
sidebar_position: 3
---

# Speaker Controller

Control your Sonos speakers from this interface.

## Device Discovery

Click the button below to discover Sonos speakers on your network:

<button id="discoverBtn" onclick="discoverSpeakers()">Discover Speakers</button>

<div id="speakersList" style="margin-top: 20px; padding: 10px; border: 1px solid #ddd; min-height: 100px;">
  <em>No speakers discovered yet. Click the button above to search.</em>
</div>

<script>
async function discoverSpeakers() {
  const button = document.getElementById('discoverBtn');
  const list = document.getElementById('speakersList');
  
  button.disabled = true;
  button.textContent = 'Discovering...';
  list.innerHTML = '<em>Searching for speakers...</em>';
  
  try {
    const response = await fetch('/api/sonos/discover', {
      method: 'POST'
    });
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const speakers = await response.json();
    
    if (speakers.length === 0) {
      list.innerHTML = '<em>No speakers found. Make sure your Sonos devices are on the same network.</em>';
    } else {
      list.innerHTML = '<h4>Found Speakers:</h4><ul>' + 
        speakers.map(speaker => `<li>${speaker.name} - ${speaker.ip}</li>`).join('') + 
        '</ul>';
    }
  } catch (error) {
    list.innerHTML = `<em style="color: red;">Error: ${error.message}</em>`;
  } finally {
    button.disabled = false;
    button.textContent = 'Discover Speakers';
  }
}
</script>

## Music Controls

<div style="margin-top: 30px;">
  <button onclick="controlMusic('play')" style="margin-right: 10px;">▶️ Play</button>
  <button onclick="controlMusic('pause')" style="margin-right: 10px;">⏸️ Pause</button>
  <button onclick="controlMusic('restart-playlist')">🔄 Restart Playlist</button>
</div>

<script>
async function controlMusic(action) {
  try {
    const response = await fetch(`/api/sonos/${action}`, {
      method: 'POST'
    });
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const result = await response.text();
    console.log(`${action}: ${result}`);
  } catch (error) {
    console.error(`Error controlling music: ${error.message}`);
  }
}
</script>
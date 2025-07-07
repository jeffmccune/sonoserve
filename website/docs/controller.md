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
          alert(`✅ ${result}`);
        })
        .catch(error => {
          console.error('Play error:', error);
          alert(`❌ Failed to play: ${error.message}`);
        })
        .finally(() => {
          playButton.disabled = false;
          playButton.textContent = '▶️ Play';
        });
    }} 
    style={{marginRight: '10px'}}
    data-action="play"
  >
    ▶️ Play
  </button>
  <button 
    onClick={() => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/pause' 
        : `http://${server}/sonos/pause`;
      
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
          alert(`✅ ${result}`);
        })
        .catch(error => {
          console.error('Pause error:', error);
          alert(`❌ Failed to pause: ${error.message}`);
        });
    }} 
    style={{marginRight: '10px'}}
  >
    ⏸️ Pause
  </button>
  <button 
    onClick={() => {
      const selectedSpeaker = document.querySelector('input[name="speaker"]:checked');
      if (!selectedSpeaker) {
        alert('Please select a speaker first');
        return;
      }
      
      const server = document.getElementById('serverInput').value || 'localhost:8080';
      const url = (window.location.host === server) 
        ? '/sonos/restart-playlist' 
        : `http://${server}/sonos/restart-playlist`;
      
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
          alert(`✅ ${result}`);
        })
        .catch(error => {
          console.error('Restart playlist error:', error);
          alert(`❌ Failed to restart playlist: ${error.message}`);
        });
    }}
  >
    🔄 Restart Playlist
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
            alert(`✅ ${result}`);
          })
          .catch(error => {
            console.error(`Preset ${num} error:`, error);
            alert(`❌ Failed to play preset ${num}: ${error.message}`);
          })
          .finally(() => {
            button.disabled = false;
            button.textContent = num;
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
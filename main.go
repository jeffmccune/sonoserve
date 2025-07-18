package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/ianr0bkny/go-sonos"
	"github.com/ianr0bkny/go-sonos/ssdp"
	"github.com/ianr0bkny/go-sonos/upnp"
)

// Build-time variables (set via ldflags)
var (
	version      = "dev"
	gitCommit    = "unknown"
	gitTreeState = "unknown"
	buildDate    = "unknown"
)

//go:embed all:music
var musicFS embed.FS

// Default speaker name from command line
var defaultSpeaker string

type SpeakerInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type Speaker struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Room    string `json:"room"`
}

type ListItem struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// Global cache of discovered speakers
var speakerCache = make(map[string]Speaker)

// Global variables for server configuration
var resourceHost string

// Global flag to track if initial discovery is complete
var initialDiscoveryComplete = false

// getLocalIP returns the local network IP address (non-loopback)
func getLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Error getting network interfaces: %v", err)
		return "localhost"
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			// We want IPv4 addresses that are not loopback
			if ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
				// Prefer private network addresses
				if ipNet.IP.IsPrivate() {
					return ipNet.IP.String()
				}
			}
		}
	}

	// Fallback: try to find any non-loopback IPv4 address
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			if ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
				return ipNet.IP.String()
			}
		}
	}

	log.Println("Warning: Could not determine local IP address, using localhost")
	return "localhost"
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Serve embedded music files
	musicSubFS, err := fs.Sub(musicFS, "music")
	if err != nil {
		log.Fatalf("Failed to create music sub filesystem: %v", err)
	}
	// Use custom handler to set proper MIME type for MP3 files
	mux.Handle("/music/", http.StripPrefix("/music/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(strings.ToLower(r.URL.Path), ".mp3") {
			w.Header().Set("Content-Type", "audio/mpeg")
		}
		http.FileServer(http.FS(musicSubFS)).ServeHTTP(w, r)
	})))

	// Serve embedded website
	websiteSubFS, err := fs.Sub(websiteFS, "build")
	if err != nil {
		log.Fatalf("Failed to create website sub filesystem: %v", err)
	}
	mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(websiteSubFS))))

	mux.HandleFunc("/", rootRedirectHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/playlist", playlistHandler)
	mux.HandleFunc("/sonos/play", playHandler)
	mux.HandleFunc("/sonos/pause", pauseHandler)
	mux.HandleFunc("/sonos/restart-playlist", restartPlaylistHandler)
	mux.HandleFunc("/sonos/queue", queueHandler)
	mux.HandleFunc("/api/sonos/discover", discoverHandler)
	mux.HandleFunc("/api/sonos/speakers", speakersHandler)
	mux.HandleFunc("/echo", echoHandler)
	mux.HandleFunc("/sonos/preset/", presetHandler)
	mux.HandleFunc("/sonos/play-pause", playPauseHandler)
	mux.HandleFunc("/sonos/next", nextTrackHandler)
	mux.HandleFunc("/sonos/previous", previousTrackHandler)
	mux.HandleFunc("/sonos/volume-up", volumeUpHandler)
	mux.HandleFunc("/sonos/volume-down", volumeDownHandler)
	mux.HandleFunc("/sonos/mute", muteHandler)

	return mux
}

func rootRedirectHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/ui/docs/controller", http.StatusTemporaryRedirect)
		return
	}
	// For any other root-level path, return 404
	http.NotFound(w, r)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if !initialDiscoveryComplete {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Initial discovery in progress\n"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

// getEmbeddedFiles returns the list of MP3 files in the embedded filesystem for a given preset
func getEmbeddedFiles(presetNum string) ([]string, error) {
	// Check if preset directory exists
	presetDir := fmt.Sprintf("music/presets/%s", presetNum)
	entries, err := musicFS.ReadDir(presetDir)
	if err != nil {
		return nil, fmt.Errorf("preset %s not found", presetNum)
	}
	
	// Collect all MP3 files from the preset directory
	var mp3Files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mp3") {
			// Decode URL-encoded filename if needed
			decodedName, err := url.QueryUnescape(entry.Name())
			if err == nil {
				mp3Files = append(mp3Files, decodedName)
			} else {
				mp3Files = append(mp3Files, entry.Name())
			}
		}
	}
	
	// Sort files alphanumerically (even if empty)
	sort.Strings(mp3Files)
	
	return mp3Files, nil
}

// getPresetPlaylistItems returns a sorted list of playlist items for a given preset
func getPresetPlaylistItems(presetNum string, scheme string) ([]ListItem, error) {
	// Get embedded files for this preset
	mp3Files, err := getEmbeddedFiles(presetNum)
	if err != nil {
		return nil, err
	}
	
	// Build playlist items
	baseURL := fmt.Sprintf("%s://%s", scheme, resourceHost)
	playlistItems := make([]ListItem, 0, len(mp3Files))
	
	for i, mp3File := range mp3Files {
		songURL := fmt.Sprintf("%s/music/presets/%s/%s", baseURL, presetNum, url.PathEscape(mp3File))
		songTitle := strings.TrimSuffix(mp3File, filepath.Ext(mp3File))
		
		item := ListItem{
			Index:    i,
			Title:    songTitle,
			Filename: mp3File,
			URL:      songURL,
		}
		playlistItems = append(playlistItems, item)
	}
	
	return playlistItems, nil
}

// playPreset handles the POST request to play a preset on a speaker
func playPreset(w http.ResponseWriter, r *http.Request, presetNum string, speakerName string) {
	log.Printf("Preset %s requested for speaker: %s", presetNum, speakerName)
	
	// Get playlist items
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	
	playlistItems, err := getPresetPlaylistItems(presetNum, scheme)
	if err != nil {
		log.Printf("Failed to get preset playlist: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[speakerName]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", speakerName), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with AV Transport and Content Directory services
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT|sonos.SVC_CONTENT_DIRECTORY)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Clear the current queue first
	log.Printf("Clearing current queue on %s", speaker.Name)
	err = s.RemoveAllTracksFromQueue(0)
	if err != nil {
		log.Printf("Warning: Failed to clear queue: %v", err)
	}
	
	// Add all MP3 files from the preset to the queue
	var addedTracks int
	for _, item := range playlistItems {
		songURL := item.URL
		songTitle := item.Title
		
		log.Printf("Adding track to queue: %s", songURL)
		
		// Add URI to queue with filename as metadata
		req := &upnp.AddURIToQueueIn{
			EnqueuedURI:         songURL,
			EnqueuedURIMetaData: fmt.Sprintf("<DIDL-Lite><item><dc:title>%s</dc:title></item></DIDL-Lite>", songTitle),
			DesiredFirstTrackNumberEnqueued: 0,
			EnqueueAsNext: false,
		}
		
		if out, err := s.AddURIToQueue(0, req); err != nil {
			log.Printf("Failed to add track %s to queue: %v", songURL, err)
			http.Error(w, "Failed to add tracks to queue", http.StatusInternalServerError)
			return
		} else {
			log.Printf("Added track %s at position %d", songURL, out.FirstTrackNumberEnqueued)
			addedTracks++
		}
	}
	
	log.Printf("Added %d tracks from preset %s to queue, setting up playback from queue", addedTracks, presetNum)
	
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
	
	log.Printf("Queue URI set successfully, starting playback...")
	
	// Start playback from the queue
	err = s.Play(0, "1")
	if err != nil {
		log.Printf("Failed to start playback: %v", err)
		http.Error(w, "Failed to start playback", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully started playing preset %s on %s", presetNum, speaker.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Playing preset %s on %s\n", presetNum, speaker.Name)))
}

func presetHandler(w http.ResponseWriter, r *http.Request) {
	// Extract preset number from path
	path := r.URL.Path
	presetNum := strings.TrimPrefix(path, "/sonos/preset/")
	if presetNum == "" || presetNum == path {
		http.Error(w, "Invalid preset path", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		// Return playlist items as JSON
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		
		playlistItems, err := getPresetPlaylistItems(presetNum, scheme)
		if err != nil {
			log.Printf("Failed to get preset playlist: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		
		response := map[string]interface{}{
			"preset":         presetNum,
			"playlist_count": len(playlistItems),
			"playlist_items": playlistItems,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		
	case http.MethodPost:
		// Parse JSON request body to get optional speaker name
		var req struct {
			Speaker string `json:"speaker"`
		}
		
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				// Body might be empty or invalid JSON, that's okay
				log.Printf("Could not parse JSON body: %v", err)
			}
		}
		
		if req.Speaker == "" {
			req.Speaker = defaultSpeaker
		}
		
		playPreset(w, r, presetNum, req.Speaker)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func playlistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	log.Println("Generating dynamic playlist...")
	
	// Use the configured resource host for external devices to reach us
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, resourceHost)
	
	// Walk the embedded music filesystem to find all MP3 files
	var songs []string
	err := fs.WalkDir(musicFS, "music", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".mp3") {
			// Convert embedded path to HTTP URL
			// Remove "music/" prefix since our HTTP handler strips it
			httpPath := strings.TrimPrefix(path, "music/")
			songURL := fmt.Sprintf("%s/music/%s", baseURL, url.PathEscape(httpPath))
			songs = append(songs, songURL)
			log.Printf("Added to playlist: %s", songURL)
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("Error walking music filesystem: %v", err)
		http.Error(w, "Failed to generate playlist", http.StatusInternalServerError)
		return
	}
	
	if len(songs) == 0 {
		log.Println("No MP3 files found in embedded filesystem")
		http.Error(w, "No songs available", http.StatusNotFound)
		return
	}
	
	// Generate M3U playlist format
	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.Header().Set("Content-Disposition", "attachment; filename=\"playlist.m3u\"")
	
	// Write M3U header
	w.Write([]byte("#EXTM3U\n"))
	
	// Write each song entry
	for _, song := range songs {
		// Extract filename for display
		filename := filepath.Base(song)
		w.Write([]byte(fmt.Sprintf("#EXTINF:-1,%s\n", filename)))
		w.Write([]byte(fmt.Sprintf("%s\n", song)))
	}
	
	log.Printf("Generated playlist with %d songs", len(songs))
}

func playHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Play requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with AV Transport and Content Directory services
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT|sonos.SVC_CONTENT_DIRECTORY)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Clear the current queue first
	log.Printf("Clearing current queue on %s", speaker.Name)
	err = s.RemoveAllTracksFromQueue(0)
	if err != nil {
		log.Printf("Warning: Failed to clear queue: %v", err)
	}
	
	// Get all MP3 files from embedded filesystem and add them to the queue
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, resourceHost)
	
	var addedTracks int
	err = fs.WalkDir(musicFS, "music", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".mp3") {
			// Convert embedded path to HTTP URL
			// Remove "music/" prefix since our HTTP handler strips it
			httpPath := strings.TrimPrefix(path, "music/")
			songURL := fmt.Sprintf("%s/music/%s", baseURL, url.PathEscape(httpPath))
			
			log.Printf("Adding track to queue: %s", songURL)
			
			// Extract filename from URL for metadata
			filename := filepath.Base(songURL)
			// Remove file extension for cleaner display
			songTitle := strings.TrimSuffix(filename, filepath.Ext(filename))
			
			// Add URI to queue with filename as metadata
			req := &upnp.AddURIToQueueIn{
				EnqueuedURI:         songURL,
				EnqueuedURIMetaData: fmt.Sprintf("<DIDL-Lite><item><dc:title>%s</dc:title></item></DIDL-Lite>", songTitle),
				DesiredFirstTrackNumberEnqueued: 0,
				EnqueueAsNext: false,
			}
			
			if out, err := s.AddURIToQueue(0, req); err != nil {
				log.Printf("Failed to add track %s to queue: %v", songURL, err)
				return err
			} else {
				log.Printf("Added track %s at position %d", songURL, out.FirstTrackNumberEnqueued)
				addedTracks++
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("Failed to add tracks to queue: %v", err)
		http.Error(w, "Failed to add tracks to queue", http.StatusInternalServerError)
		return
	}
	
	if addedTracks == 0 {
		log.Println("No MP3 files found to add to queue")
		http.Error(w, "No songs available", http.StatusNotFound)
		return
	}
	
	log.Printf("Added %d tracks to queue, setting up playback from queue", addedTracks)
	
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
	
	log.Printf("Queue URI set successfully, starting playback...")
	
	// Start playback from the queue
	// Play requires (instanceID, speed)
	err = s.Play(0, "1")
	if err != nil {
		log.Printf("Failed to start playback: %v", err)
		http.Error(w, "Failed to start playback", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully started playback on %s", speaker.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Playing playlist on %s\n", speaker.Name)))
}

func queueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Speaker string `json:"speaker"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding JSON request: %v", err)
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}

	log.Printf("Queue requested for speaker: %s", req.Speaker)

	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, "Speaker not found", http.StatusNotFound)
		return
	}

	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}

	// Create Sonos connection with Content Directory service for queue browsing
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_CONTENT_DIRECTORY)
	
	queueContents, err := s.GetQueueContents()
	if err != nil {
		log.Printf("Error getting queue contents: %v", err)
		http.Error(w, "Failed to get queue contents", http.StatusInternalServerError)
		return
	}

	// Extract detailed information from queue items for debugging
	var queueItems []map[string]interface{}
	for i, item := range queueContents {
		queueItem := map[string]interface{}{
			"index":          i,
			"id":             item.ID(),
			"title":          item.Title(),
			"uri":            item.Res(),
			"creator":        item.Creator(),
			"album":          item.Album(),
			"track_number":   item.OriginalTrackNumber(),
			"class":          item.Class(),
			"album_art_uri":  item.AlbumArtURI(),
			"parent_id":      item.ParentID(),
			"restricted":     item.Restricted(),
		}
		queueItems = append(queueItems, queueItem)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"speaker":        req.Speaker,
		"queue_length":   len(queueContents),
		"queue_items":    queueItems,
	})
}

func pauseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Pause requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with AV Transport service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Pause playback
	err = s.Pause(0)
	if err != nil {
		log.Printf("Failed to pause playback: %v", err)
		http.Error(w, "Failed to pause playback", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully paused playback on %s", speaker.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Paused %s\n", speaker.Name)))
}

func restartPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Restart playlist requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with AV Transport service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Seek to the beginning of the current track (position 0)
	err = s.Seek(0, "TRACK_NR", "1")
	if err != nil {
		log.Printf("Failed to restart playlist: %v", err)
		http.Error(w, "Failed to restart playlist", http.StatusInternalServerError)
		return
	}
	
	// Start playing from the beginning
	err = s.Play(0, "1")
	if err != nil {
		log.Printf("Failed to start playback: %v", err)
		http.Error(w, "Failed to start playback", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully restarted playlist on %s", speaker.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Playlist restarted on %s\n", speaker.Name)))
}

func discoverSonosDevices() ([]SpeakerInfo, error) {
	var speakers []SpeakerInfo
	
	// Create SSDP manager
	mgr := ssdp.MakeManager()
	defer mgr.Close()
	
	// Get all available network interfaces using Go standard library
	netInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}
	
	// Filter for suitable interfaces and extract names
	var interfaceNames []string
	for _, iface := range netInterfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		// Check if interface has IPv4 addresses
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		hasIPv4 := false
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				hasIPv4 = true
				break
			}
		}
		
		if hasIPv4 {
			interfaceNames = append(interfaceNames, iface.Name)
		}
	}
	
	if len(interfaceNames) == 0 {
		return nil, fmt.Errorf("no suitable network interfaces found")
	}
	
	log.Printf("Found %d suitable network interfaces: %v", len(interfaceNames), interfaceNames)
	
	for _, iface := range interfaceNames {
		log.Printf("Trying discovery on interface: %s", iface)
		err := mgr.Discover(iface, "1900", false)
		if err != nil {
			log.Printf("Discovery error on %s: %v", iface, err)
			continue
		}
		
		// Give discovery some time to complete
		time.Sleep(2 * time.Second)
		
		// Get all discovered devices
		devices := mgr.Devices()
		log.Printf("Found %d devices on %s", len(devices), iface)
		
		// Track unique IPs to avoid duplicates (same device may have multiple services)
		seenIPs := make(map[string]bool)
		
		for _, device := range devices {
			// Check if this is a Sonos device
			if strings.Contains(strings.ToLower(device.Product()), "sonos") {
				ip := extractIPFromLocation(device.Location())
				if ip != "" && !seenIPs[ip] {
					seenIPs[ip] = true
					
					// Try to connect to get the actual room name and device name
					roomName, deviceName := getSonosRoomName(ip)
					if roomName == "" {
						roomName = device.Name() // fallback to device name
					}
					if deviceName == "" {
						deviceName = roomName
					}
					
					// Store in cache
					speaker := Speaker{
						Name:    deviceName,
						Address: ip,
						Room:    roomName,
					}
					speakerCache[deviceName] = speaker
					
					speakers = append(speakers, SpeakerInfo{
						Name: deviceName,
						IP:   ip,
					})
					log.Printf("Found Sonos device: %s (room: %s) at %s", deviceName, roomName, ip)
				}
			}
		}
		
		// If we found some speakers, no need to try other interfaces
		if len(speakers) > 0 {
			break
		}
	}
	
	return speakers, nil
}

func getSonosRoomName(ip string) (string, string) {
	log.Printf("Getting room name for Sonos device at %s", ip)
	
	// Try to find the device by creating it manually using the known IP
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", ip)
	
	// Try to describe the device at this location to get UPnP services
	if svcMap, err := upnp.Describe(ssdp.Location(locationURL)); err != nil {
		log.Printf("Failed to describe device at %s: %v", ip, err)
		return "Unknown Room", "Sonos Speaker"
	} else {
		// Create Sonos connection WITHOUT reactor to avoid HTTP handler conflicts
		// Pass nil reactor and only enable device properties service
		s := sonos.MakeSonos(svcMap, nil, sonos.SVC_DEVICE_PROPERTIES)
		if s == nil {
			log.Printf("Failed to create Sonos connection for %s", ip)
			return "Unknown Room", "Sonos Speaker"
		}
		
		// Get zone attributes - this returns (currentZoneName, currentIcon, error)
		if currentZoneName, _, err := s.GetZoneAttributes(); err != nil {
			log.Printf("Failed to get zone attributes from %s: %v", ip, err)
			return "Unknown Room", "Sonos Speaker"
		} else {
			roomName := currentZoneName
			deviceName := currentZoneName // Use zone name as device name
			
			if roomName == "" {
				roomName = "Unknown Room" 
				deviceName = "Sonos Speaker"
			}
			
			log.Printf("Found Sonos device: room='%s', device='%s'", roomName, deviceName)
			return roomName, deviceName
		}
	}
}

func extractIPFromLocation(location ssdp.Location) string {
	// The location is typically a URL like "http://192.168.4.100:1400/xml/device_description.xml"
	// Convert location to string - it should implement fmt.Stringer or be a string type
	locationStr := fmt.Sprintf("%v", location)
	if locationStr == "" {
		return ""
	}
	
	parsed, err := url.Parse(locationStr)
	if err != nil {
		log.Printf("Error parsing location URL: %v", err)
		return ""
	}
	
	// Extract just the host part (without port)
	host := parsed.Hostname()
	return host
}

func discoverHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	log.Println("Discovering Sonos devices...")
	
	speakers, err := discoverSonosDevices()
	if err != nil {
		log.Printf("Discovery error: %v", err)
		http.Error(w, "Discovery failed", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Discovery completed, found %d speakers", len(speakers))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(speakers)
}

func speakersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	log.Println("Getting cached speakers...")
	
	// Convert speakerCache map to slice for JSON response
	var speakers []Speaker
	for _, speaker := range speakerCache {
		speakers = append(speakers, speaker)
	}
	
	log.Printf("Returning %d cached speakers", len(speakers))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(speakers)
}

func playPauseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Play/Pause toggle requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with AV Transport service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Get current transport info to determine play state
	transportInfo, err := s.GetTransportInfo(0)
	if err != nil {
		log.Printf("Failed to get transport info: %v", err)
		http.Error(w, "Failed to get playback state", http.StatusInternalServerError)
		return
	}
	
	// Toggle play/pause based on current state
	if transportInfo.CurrentTransportState == "PLAYING" {
		err = s.Pause(0)
		if err != nil {
			log.Printf("Failed to pause playback: %v", err)
			http.Error(w, "Failed to pause playback", http.StatusInternalServerError)
			return
		}
		log.Printf("Successfully paused playback on %s", speaker.Name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Paused %s\n", speaker.Name)))
	} else {
		err = s.Play(0, "1")
		if err != nil {
			log.Printf("Failed to start playback: %v", err)
			http.Error(w, "Failed to start playback", http.StatusInternalServerError)
			return
		}
		log.Printf("Successfully started playback on %s", speaker.Name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Playing on %s\n", speaker.Name)))
	}
}

func nextTrackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Next track requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with AV Transport service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Move to next track
	err = s.Next(0)
	if err != nil {
		log.Printf("Failed to skip to next track: %v", err)
		http.Error(w, "Failed to skip to next track", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully skipped to next track on %s", speaker.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Next track on %s\n", speaker.Name)))
}

func previousTrackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Previous track requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with AV Transport service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_AV_TRANSPORT)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Move to previous track
	err = s.Previous(0)
	if err != nil {
		log.Printf("Failed to skip to previous track: %v", err)
		http.Error(w, "Failed to skip to previous track", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully skipped to previous track on %s", speaker.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Previous track on %s\n", speaker.Name)))
}

func volumeUpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Volume up requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with Rendering Control service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_RENDERING_CONTROL)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Get current volume
	currentVolume, err := s.GetVolume(0, "Master")
	if err != nil {
		log.Printf("Failed to get current volume: %v", err)
		http.Error(w, "Failed to get volume", http.StatusInternalServerError)
		return
	}
	
	// Increase volume by 5%, max 100
	newVolume := currentVolume + 5
	if newVolume > 100 {
		newVolume = 100
	}
	
	// Set new volume
	err = s.SetVolume(0, "Master", newVolume)
	if err != nil {
		log.Printf("Failed to set volume: %v", err)
		http.Error(w, "Failed to set volume", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully increased volume on %s from %d to %d", speaker.Name, currentVolume, newVolume)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Volume increased to %d on %s\n", newVolume, speaker.Name)))
}

func volumeDownHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Volume down requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with Rendering Control service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_RENDERING_CONTROL)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Get current volume
	currentVolume, err := s.GetVolume(0, "Master")
	if err != nil {
		log.Printf("Failed to get current volume: %v", err)
		http.Error(w, "Failed to get volume", http.StatusInternalServerError)
		return
	}
	
	// Decrease volume by 5%, min 0
	newVolume := currentVolume - 5
	if newVolume < 0 {
		newVolume = 0
	}
	
	// Set new volume
	err = s.SetVolume(0, "Master", newVolume)
	if err != nil {
		log.Printf("Failed to set volume: %v", err)
		http.Error(w, "Failed to set volume", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Successfully decreased volume on %s from %d to %d", speaker.Name, currentVolume, newVolume)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Volume decreased to %d on %s\n", newVolume, speaker.Name)))
}

func muteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse JSON request body to get optional speaker name
	var req struct {
		Speaker string `json:"speaker"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body might be empty or invalid JSON, that's okay
			log.Printf("Could not parse JSON body: %v", err)
		}
	}
	
	if req.Speaker == "" {
		req.Speaker = defaultSpeaker
	}
	
	log.Printf("Mute toggle requested for speaker: %s", req.Speaker)
	
	// Find the speaker in our cache
	speaker, exists := speakerCache[req.Speaker]
	if !exists {
		http.Error(w, fmt.Sprintf("Speaker '%s' not found", req.Speaker), http.StatusNotFound)
		return
	}
	
	// Connect to Sonos device
	locationURL := fmt.Sprintf("http://%s:1400/xml/device_description.xml", speaker.Address)
	
	svcMap, err := upnp.Describe(ssdp.Location(locationURL))
	if err != nil {
		log.Printf("Failed to describe Sonos device: %v", err)
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Create Sonos connection with Rendering Control service
	s := sonos.MakeSonos(svcMap, nil, sonos.SVC_RENDERING_CONTROL)
	if s == nil {
		log.Printf("Failed to create Sonos connection")
		http.Error(w, "Failed to connect to speaker", http.StatusInternalServerError)
		return
	}
	
	// Get current mute state
	currentMute, err := s.GetMute(0, "Master")
	if err != nil {
		log.Printf("Failed to get current mute state: %v", err)
		http.Error(w, "Failed to get mute state", http.StatusInternalServerError)
		return
	}
	
	// Toggle mute state
	newMute := !currentMute
	err = s.SetMute(0, "Master", newMute)
	if err != nil {
		log.Printf("Failed to set mute state: %v", err)
		http.Error(w, "Failed to set mute state", http.StatusInternalServerError)
		return
	}
	
	muteStatus := "unmuted"
	if newMute {
		muteStatus = "muted"
	}
	
	log.Printf("Successfully %s %s", muteStatus, speaker.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Speaker %s %s\n", speaker.Name, muteStatus)))
}

func printVersion() {
	fmt.Printf("sonoserve version %s\n", version)
	fmt.Printf("  git commit: %s\n", gitCommit)
	fmt.Printf("  git tree state: %s\n", gitTreeState)
	fmt.Printf("  build date: %s\n", buildDate)
}

func main() {
	// Determine default resource host (IP address for external devices to reach us)
	defaultResourceHost := getLocalIP() + ":8080"
	
	var (
		showVersion    = flag.Bool("version", false, "show version information")
		listFiles      = flag.String("list-files", "", "list embedded files for a preset (e.g., -list-files=5)")
		addr           = flag.String("addr", ":8080", "server listen address (interface:port)")
		resourceHostPtr = flag.String("resource-host", defaultResourceHost, "host:port for external devices to fetch resources from this server")
		defaultSpeakerPtr = flag.String("default-speaker", "Kids Room", "default speaker name to use when not specified")
	)
	flag.Parse()
	
	// Set global variables
	resourceHost = *resourceHostPtr
	defaultSpeaker = *defaultSpeakerPtr

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	if *listFiles != "" {
		files, err := getEmbeddedFiles(*listFiles)
		if err != nil {
			log.Fatalf("Error listing files for preset %s: %v", *listFiles, err)
		}
		jsonOutput, err := json.Marshal(files)
		if err != nil {
			log.Fatalf("Error encoding files to JSON: %v", err)
		}
		fmt.Println(string(jsonOutput))
		os.Exit(0)
	}

	log.Printf("Starting sonoserve %s", version)
	if gitCommit != "unknown" {
		log.Printf("Git commit: %s", gitCommit)
	}
	log.Printf("Listen address: %s", *addr)
	log.Printf("Resource host: %s", resourceHost)

	// Perform initial Sonos discovery on startup
	log.Println("Performing initial Sonos discovery...")
	go func() {
		speakers, err := discoverSonosDevices()
		if err != nil {
			log.Printf("Startup discovery failed: %v", err)
		} else {
			log.Printf("Startup discovery completed, found %d speakers", len(speakers))
			for _, speaker := range speakers {
				log.Printf("  - %s at %s", speaker.Name, speaker.IP)
			}
		}
		// Mark initial discovery as complete
		initialDiscoveryComplete = true
		log.Println("Initial discovery complete, health endpoint now ready")
	}()

	mux := setupRoutes()

	srv := &http.Server{
		Addr:    *addr,
		Handler: corsMiddleware(mux),
	}

	go func() {
		log.Printf("Server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
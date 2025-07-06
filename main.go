package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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

type SpeakerInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type Speaker struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Room    string `json:"room"`
}

// Global cache of discovered speakers
var speakerCache = make(map[string]Speaker)

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
	mux.Handle("/music/", http.StripPrefix("/music/", http.FileServer(http.FS(musicSubFS))))

	// Serve embedded website
	websiteSubFS, err := fs.Sub(websiteFS, "build")
	if err != nil {
		log.Fatalf("Failed to create website sub filesystem: %v", err)
	}
	mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(websiteSubFS))))

	mux.HandleFunc("/", rootRedirectHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/sonos/play", playHandler)
	mux.HandleFunc("/sonos/pause", pauseHandler)
	mux.HandleFunc("/sonos/restart-playlist", restartPlaylistHandler)
	mux.HandleFunc("/api/sonos/discover", discoverHandler)
	mux.HandleFunc("/api/sonos/speakers", speakersHandler)

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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func playHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Println("Play requested")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Playing\n"))
}

func pauseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Println("Pause requested")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Paused\n"))
}

func restartPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Println("Restart playlist requested")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Playlist restarted\n"))
}

func discoverSonosDevices() ([]SpeakerInfo, error) {
	var speakers []SpeakerInfo
	
	// Create SSDP manager
	mgr := ssdp.MakeManager()
	defer mgr.Close()
	
	// Try common network interfaces
	interfaces := []string{"en0", "eth0", "wlan0", "en1"}
	
	for _, iface := range interfaces {
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

func printVersion() {
	fmt.Printf("sonoserve version %s\n", version)
	fmt.Printf("  git commit: %s\n", gitCommit)
	fmt.Printf("  git tree state: %s\n", gitTreeState)
	fmt.Printf("  build date: %s\n", buildDate)
}

func main() {
	var (
		showVersion = flag.Bool("version", false, "show version information")
		addr        = flag.String("addr", ":8080", "server address")
	)
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	log.Printf("Starting sonoserve %s", version)
	if gitCommit != "unknown" {
		log.Printf("Git commit: %s", gitCommit)
	}

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
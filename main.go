package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed all:music
var musicFS embed.FS

type SpeakerInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
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
	websiteSubFS, err := fs.Sub(websiteFS, "website/build")
	if err != nil {
		log.Fatalf("Failed to create website sub filesystem: %v", err)
	}
	mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(websiteSubFS))))

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/sonos/play", playHandler)
	mux.HandleFunc("/sonos/pause", pauseHandler)
	mux.HandleFunc("/sonos/restart-playlist", restartPlaylistHandler)
	mux.HandleFunc("/api/sonos/discover", discoverHandler)

	return mux
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

func discoverHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	log.Println("Discovering Sonos devices...")
	
	var speakers []SpeakerInfo
	
	// For now, return a mock speaker for testing
	// TODO: Implement actual Sonos discovery
	speakers = append(speakers, SpeakerInfo{
		Name: "Test Sonos Speaker",
		IP:   "192.168.1.100",
	})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(speakers)
}

func main() {
	mux := setupRoutes()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Printf("Starting server on %s", srv.Addr)
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
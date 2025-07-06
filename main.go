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

	"github.com/ianr0bkny/go-sonos"
	"github.com/ianr0bkny/go-sonos/ssdp"
)

//go:embed music/*
var musicFS embed.FS

func main() {
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

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK\n"))
	})

	mux.HandleFunc("/sonos/play", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		log.Println("Play requested")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Playing\n"))
	})

	mux.HandleFunc("/sonos/pause", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		log.Println("Pause requested")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Paused\n"))
	})

	mux.HandleFunc("/sonos/restart-playlist", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		log.Println("Restart playlist requested")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Playlist restarted\n"))
	})

	mux.HandleFunc("/api/sonos/discover", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		log.Println("Discovering Sonos devices...")
		
		// Use SSDP to discover Sonos devices
		mgr := ssdp.MakeManager()
		devices := mgr.Discover("eth0", "11209", false)
		
		type SpeakerInfo struct {
			Name string `json:"name"`
			IP   string `json:"ip"`
		}
		
		var speakers []SpeakerInfo
		
		for _, device := range devices {
			s := sonos.Connect(device, nil, sonos.SVC_ALL)
			if s != nil {
				// Get device info
				attrs, err := s.GetAttributes()
				if err == nil {
					speakers = append(speakers, SpeakerInfo{
						Name: attrs["CurrentZoneName"],
						IP:   device.String(),
					})
				}
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(speakers)
	})

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
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "OK\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestPlayHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "valid POST request",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectedBody:   "Playing\n",
		},
		{
			name:           "invalid GET request",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed\n",
		},
		{
			name:           "invalid PUT request",
			method:         "PUT",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "/sonos/play", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(playHandler)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			if rr.Body.String() != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v",
					rr.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestPauseHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "valid POST request",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectedBody:   "Paused\n",
		},
		{
			name:           "invalid GET request",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "/sonos/pause", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(pauseHandler)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			if rr.Body.String() != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v",
					rr.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestRestartPlaylistHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "valid POST request",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectedBody:   "Playlist restarted\n",
		},
		{
			name:           "invalid DELETE request",
			method:         "DELETE",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "/sonos/restart-playlist", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(restartPlaylistHandler)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			if rr.Body.String() != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v",
					rr.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestDiscoverHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "valid POST request",
			method:         "POST",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid GET request",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "/api/sonos/discover", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(discoverHandler)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			if tt.method == "POST" {
				// Verify JSON response structure
				var speakers []SpeakerInfo
				err := json.Unmarshal(rr.Body.Bytes(), &speakers)
				if err != nil {
					t.Errorf("failed to unmarshal response: %v", err)
				}

				if len(speakers) != 1 {
					t.Errorf("expected 1 speaker, got %d", len(speakers))
				}

				if speakers[0].Name != "Test Sonos Speaker" {
					t.Errorf("expected speaker name 'Test Sonos Speaker', got %s", speakers[0].Name)
				}

				if speakers[0].IP != "192.168.1.100" {
					t.Errorf("expected speaker IP '192.168.1.100', got %s", speakers[0].IP)
				}

				// Verify Content-Type header
				expectedContentType := "application/json"
				actualContentType := rr.Header().Get("Content-Type")
				if !strings.Contains(actualContentType, expectedContentType) {
					t.Errorf("expected Content-Type to contain %s, got %s",
						expectedContentType, actualContentType)
				}
			}
		})
	}
}

func TestSetupRoutes(t *testing.T) {
	mux := setupRoutes()
	
	// Test that all routes are properly registered by making requests
	tests := []struct {
		name         string
		method       string
		path         string
		body         *bytes.Buffer
		expectedCode int
	}{
		{
			name:         "health endpoint",
			method:       "GET",
			path:         "/health",
			body:         nil,
			expectedCode: http.StatusOK,
		},
		{
			name:         "play endpoint",
			method:       "POST",
			path:         "/sonos/play",
			body:         bytes.NewBuffer([]byte{}),
			expectedCode: http.StatusOK,
		},
		{
			name:         "pause endpoint",
			method:       "POST",
			path:         "/sonos/pause",
			body:         bytes.NewBuffer([]byte{}),
			expectedCode: http.StatusOK,
		},
		{
			name:         "restart playlist endpoint",
			method:       "POST",
			path:         "/sonos/restart-playlist",
			body:         bytes.NewBuffer([]byte{}),
			expectedCode: http.StatusOK,
		},
		{
			name:         "discover endpoint",
			method:       "POST",
			path:         "/api/sonos/discover",
			body:         bytes.NewBuffer([]byte{}),
			expectedCode: http.StatusOK,
		},
		{
			name:         "music file endpoint",
			method:       "GET",
			path:         "/music/sample.mp3",
			body:         nil,
			expectedCode: http.StatusOK,
		},
		{
			name:         "UI endpoint",
			method:       "GET",
			path:         "/ui/",
			body:         nil,
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			var err error

			if tt.body != nil {
				req, err = http.NewRequest(tt.method, tt.path, tt.body)
			} else {
				req, err = http.NewRequest(tt.method, tt.path, nil)
			}
			
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedCode {
				t.Errorf("route %s returned wrong status code: got %v want %v",
					tt.path, status, tt.expectedCode)
			}
		})
	}
}

func TestMusicFileServing(t *testing.T) {
	mux := setupRoutes()
	
	req, err := http.NewRequest("GET", "/music/sample.mp3", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("music file handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check Content-Type header for MP3
	expectedContentType := "audio/mpeg"
	actualContentType := rr.Header().Get("Content-Type")
	if actualContentType != expectedContentType {
		t.Errorf("expected Content-Type %s, got %s",
			expectedContentType, actualContentType)
	}

	// Check that we get some content
	if rr.Body.Len() == 0 {
		t.Error("expected non-empty response body for music file")
	}
}

func TestWebsiteServing(t *testing.T) {
	mux := setupRoutes()
	
	req, err := http.NewRequest("GET", "/ui/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("website handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check Content-Type header for HTML
	expectedContentType := "text/html"
	actualContentType := rr.Header().Get("Content-Type")
	if !strings.Contains(actualContentType, expectedContentType) {
		t.Errorf("expected Content-Type to contain %s, got %s",
			expectedContentType, actualContentType)
	}

	// Check that we get some HTML content
	if rr.Body.Len() == 0 {
		t.Error("expected non-empty response body for website")
	}

	// Basic check that it looks like HTML
	body := rr.Body.String()
	if !strings.Contains(body, "<html") && !strings.Contains(body, "<!DOCTYPE") {
		t.Error("response body doesn't appear to be HTML")
	}
}

func TestNonExistentRoutes(t *testing.T) {
	mux := setupRoutes()
	
	tests := []struct {
		name string
		path string
	}{
		{"root path", "/"},
		{"non-existent API", "/api/non-existent"},
		{"non-existent music file", "/music/non-existent.mp3"},
		{"non-existent UI path", "/ui/non-existent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusNotFound {
				t.Errorf("route %s should return 404, got %v", tt.path, status)
			}
		})
	}
}
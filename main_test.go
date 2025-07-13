package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPresetHandlerGET(t *testing.T) {
	// Test GET method for preset 5
	req, err := http.NewRequest("GET", "/sonos/preset/5", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(presetHandler)

	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check Content-Type header
	expectedContentType := "application/json"
	actualContentType := rr.Header().Get("Content-Type")
	if !strings.Contains(actualContentType, expectedContentType) {
		t.Errorf("expected Content-Type to contain %s, got %s",
			expectedContentType, actualContentType)
	}

	// Parse JSON response
	var response struct {
		Preset        string              `json:"preset"`
		PlaylistCount int                 `json:"playlist_count"`
		PlaylistItems []map[string]string `json:"playlist_items"`
	}

	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify preset number
	if response.Preset != "5" {
		t.Errorf("expected preset '5', got '%s'", response.Preset)
	}

	// Verify we have playlist items
	if response.PlaylistCount == 0 {
		t.Error("expected playlist_count > 0")
	}

	// Verify playlist_count matches actual items
	if response.PlaylistCount != len(response.PlaylistItems) {
		t.Errorf("playlist_count (%d) doesn't match actual items (%d)",
			response.PlaylistCount, len(response.PlaylistItems))
	}

	// expectedFiles represents the mp3 files in the embedded filesystem for preset 5,
	// sorted alpha-numerically. This should match what getEmbeddedFiles returns.
	expectedFiles, err := getEmbeddedFiles("5")
	if err != nil {
		t.Fatalf("failed to get embedded files: %v", err)
	}

	if len(response.PlaylistItems) != len(expectedFiles) {
		t.Errorf("expected %d items, got %d", len(expectedFiles), len(response.PlaylistItems))
	}

	// Verify each item is in sorted order and has required fields
	for i, item := range response.PlaylistItems {
		// Check index
		if item["index"] != fmt.Sprintf("%d", i) {
			t.Errorf("item %d has wrong index: %s", i, item["index"])
		}

		// Check filename matches expected sorted order
		if i < len(expectedFiles) && item["filename"] != expectedFiles[i] {
			t.Errorf("item %d filename mismatch: expected %s, got %s",
				i, expectedFiles[i], item["filename"])
		}

		// Check required fields exist
		if _, ok := item["title"]; !ok {
			t.Errorf("item %d missing 'title' field", i)
		}
		if _, ok := item["url"]; !ok {
			t.Errorf("item %d missing 'url' field", i)
		}

		// Verify URL format
		if !strings.Contains(item["url"], "/music/presets/5/") {
			t.Errorf("item %d URL doesn't contain expected path: %s", i, item["url"])
		}
	}
}

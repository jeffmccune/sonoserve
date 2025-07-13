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

func TestAllPresetDirectories(t *testing.T) {
	// Walk the music/presets/ folder to discover all preset directories
	entries, err := musicFS.ReadDir("music/presets")
	if err != nil {
		t.Fatalf("failed to read presets directory: %v", err)
	}

	var presetDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			presetDirs = append(presetDirs, entry.Name())
		}
	}

	if len(presetDirs) == 0 {
		t.Fatal("no preset directories found")
	}

	// Test each discovered preset directory
	for _, presetNum := range presetDirs {
		t.Run(fmt.Sprintf("preset_%s", presetNum), func(t *testing.T) {
			// Get files from the directory using getEmbeddedFiles
			expectedFiles, err := getEmbeddedFiles(presetNum)
			if err != nil {
				// Skip test for empty preset directories (like preset 6 which has no songs yet)
				if strings.Contains(err.Error(), "no songs in preset") {
					t.Skipf("Skipping preset %s: %v", presetNum, err)
					return
				}
				t.Fatalf("failed to get embedded files for preset %s: %v", presetNum, err)
			}

			// Make HTTP request to GET endpoint
			req, err := http.NewRequest("GET", fmt.Sprintf("/sonos/preset/%s", presetNum), nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(presetHandler)
			handler.ServeHTTP(rr, req)

			// Check status code
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
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

			// Verify preset number matches
			if response.Preset != presetNum {
				t.Errorf("expected preset '%s', got '%s'", presetNum, response.Preset)
			}

			// Verify playlist count matches expected files
			if response.PlaylistCount != len(expectedFiles) {
				t.Errorf("playlist_count (%d) doesn't match expected files (%d)", response.PlaylistCount, len(expectedFiles))
			}

			// Verify actual playlist items match expected files
			if len(response.PlaylistItems) != len(expectedFiles) {
				t.Errorf("playlist items count (%d) doesn't match expected files (%d)", len(response.PlaylistItems), len(expectedFiles))
			}

			// Verify each playlist item matches the corresponding file
			for i, expectedFile := range expectedFiles {
				if i >= len(response.PlaylistItems) {
					t.Errorf("missing playlist item for file: %s", expectedFile)
					continue
				}

				item := response.PlaylistItems[i]

				// Check filename matches
				if item["filename"] != expectedFile {
					t.Errorf("item %d filename mismatch: expected %s, got %s", i, expectedFile, item["filename"])
				}

				// Check required fields exist
				if _, ok := item["title"]; !ok {
					t.Errorf("item %d missing 'title' field", i)
				}
				if _, ok := item["url"]; !ok {
					t.Errorf("item %d missing 'url' field", i)
				}
				if _, ok := item["index"]; !ok {
					t.Errorf("item %d missing 'index' field", i)
				}

				// Verify URL contains correct preset path
				expectedPath := fmt.Sprintf("/music/presets/%s/", presetNum)
				if !strings.Contains(item["url"], expectedPath) {
					t.Errorf("item %d URL doesn't contain expected path %s: %s", i, expectedPath, item["url"])
				}
			}
		})
	}
}

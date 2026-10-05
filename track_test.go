package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeffmccune/sonoserve/internal/mp3tag"
)

// TestMusicEmbedded checks every file in the music folder, including the
// YAML tags and JPEG artwork files, is embedded in the executable.
func TestMusicEmbedded(t *testing.T) {
	n := 0
	err := filepath.WalkDir("music", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		n++
		if _, err := fs.Stat(musicFS, filepath.ToSlash(p)); err != nil {
			t.Errorf("%s is not embedded: %v", p, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Skip("no music files")
	}
}

func TestPresetTrack(t *testing.T) {
	tests := []struct {
		uri, preset, filename string
		ok                    bool
	}{
		{"http://host:8080/music/presets/7/03-Come%20As%20You%20Are.mp3", "7", "03-Come As You Are.mp3", true},
		{"http://host:8080/music/presets/a/01.mp3", "a", "01.mp3", true},
		{"http://host:8080/music/other/01.mp3", "", "", false},
		{"http://host:8080/music/presets/7/sub/01.mp3", "", "", false},
		{"x-sonos-spotify:spotify%3atrack%3a123", "", "", false},
	}
	for _, tt := range tests {
		preset, filename, ok := presetTrack(tt.uri)
		if preset != tt.preset || filename != tt.filename || ok != tt.ok {
			t.Errorf("presetTrack(%q) = %q, %q, %v, want %q, %q, %v", tt.uri, preset, filename, ok, tt.preset, tt.filename, tt.ok)
		}
	}
}

// TestNewTrackResponse checks the response for every embedded preset mp3 file
// with a YAML tags file has the title and album of its tags, and the track's
// own artwork when it has a JPEG file.
func TestNewTrackResponse(t *testing.T) {
	mp3s, err := fs.Glob(musicFS, "music/presets/*/*.mp3")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, p := range mp3s {
		dir, filename := path.Split(p)
		preset := path.Base(dir)
		tagsFile, artworkFile, err := mp3tag.Find(p)
		if err != nil {
			t.Fatal(err)
		}
		if tagsFile == "" {
			continue
		}
		tags, err := mp3tag.ReadTags(tagsFile)
		if err != nil {
			t.Fatal(err)
		}
		checked++

		resp := newTrackResponse("Kids Room", preset, filename)
		if resp.Title != tags.Title || resp.Album != tags.Album {
			t.Errorf("%s: title %q album %q, want %q %q", p, resp.Title, resp.Album, tags.Title, tags.Album)
		}
		var wantURL string
		if artworkFile != "" {
			wantURL = artworkPath(preset, filepath.Base(artworkFile))
		}
		if resp.ArtworkURL != wantURL {
			t.Errorf("%s: artwork_url %q, want %q", p, resp.ArtworkURL, wantURL)
		}
		if wantURL != "" && (resp.ArtworkETag == "" || resp.ArtworkTimeoutSeconds == nil) {
			t.Errorf("%s: missing artwork etag or timeout", p)
		}
	}
	if checked == 0 {
		t.Skip("no preset mp3 files with tags")
	}
}

func TestNewTrackResponseNotPreset(t *testing.T) {
	resp := newTrackResponse("Kids Room", "", "Some Song.mp3")
	if resp.Title != "Some Song" || resp.ArtworkURL != "" {
		t.Errorf("got %+v", resp)
	}
}

func TestTrackArtworkHandler(t *testing.T) {
	const key = "music/presets/zz/01-A Song.jpg"
	artworkMu.Lock()
	artworkCache[key] = &Artwork{Data: []byte("track jpeg"), ETag: "def456"}
	artworkMu.Unlock()
	defer func() {
		artworkMu.Lock()
		delete(artworkCache, key)
		artworkMu.Unlock()
	}()

	url := artworkPath("zz", "01-A Song.jpg")
	if !strings.HasPrefix(url, "/sonos/artwork/zz/") || strings.Contains(url, " ") {
		t.Fatalf("artworkPath = %q", url)
	}
	rr := httptest.NewRecorder()
	artworkHandler(rr, httptest.NewRequest("GET", url, nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "track jpeg" {
		t.Fatalf("got %d %q", rr.Code, rr.Body.String())
	}

	for _, bad := range []string{"/sonos/artwork/zz/01.png", "/sonos/artwork/zz/a/b.jpg"} {
		rr = httptest.NewRecorder()
		artworkHandler(rr, httptest.NewRequest("GET", bad, nil))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", bad, rr.Code)
		}
	}
	rr = httptest.NewRecorder()
	artworkHandler(rr, httptest.NewRequest("GET", "/sonos/artwork/zz/missing.jpg", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("missing: got %d, want 404", rr.Code)
	}
}

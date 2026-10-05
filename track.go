package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/bogem/id3v2/v2"
	"github.com/ianr0bkny/go-sonos"
)

// TrackResponse is the response body of requests that play something: a
// preset, next, previous, play, and play-pause when it resumes playback. It
// describes the track now playing so the CardPuter can show it.
type TrackResponse struct {
	// Preset is the preset the track belongs to, empty if the track is not
	// an embedded preset file.
	Preset   string `json:"preset"`
	Speaker  string `json:"speaker"`
	Filename string `json:"filename"`
	// Title, Album, and Artist come from the ID3 tag of the mp3 file. Title
	// falls back to the file name without its extension.
	Title  string `json:"title"`
	Album  string `json:"album,omitempty"`
	Artist string `json:"artist,omitempty"`
	// ArtworkURL is the URL path of the track artwork, or the preset
	// artwork.jpg if the track has none. Empty if neither exists.
	ArtworkURL string `json:"artwork_url,omitempty"`
	// ArtworkETag is the checksum of the artwork served at ArtworkURL.
	ArtworkETag string `json:"artwork_etag,omitempty"`
	// ArtworkTimeoutSeconds is how long the CardPuter shows the artwork
	// before turning off the screen, zero to keep it on.
	ArtworkTimeoutSeconds *int `json:"artwork_timeout_seconds,omitempty"`
}

// TrackTags is the ID3 metadata of an embedded mp3 file.
type TrackTags struct {
	Title  string
	Album  string
	Artist string
}

var (
	trackTagsMu sync.Mutex
	// trackTagsCache maps the embedded path of an mp3 file to its tags.
	trackTagsCache = make(map[string]TrackTags)
)

// getTrackTags returns the ID3 tags of the embedded mp3 file at p, empty if
// the file has no tag. Embedded files never change so results are cached.
func getTrackTags(p string) (TrackTags, error) {
	trackTagsMu.Lock()
	defer trackTagsMu.Unlock()
	if tags, ok := trackTagsCache[p]; ok {
		return tags, nil
	}
	f, err := musicFS.Open(p)
	if err != nil {
		return TrackTags{}, err
	}
	defer f.Close()
	tag, err := id3v2.ParseReader(f, id3v2.Options{Parse: true, ParseFrames: []string{"Title", "Album/Movie/Show title", "Artist"}})
	if err != nil {
		return TrackTags{}, err
	}
	tags := TrackTags{
		Title:  strings.TrimSpace(tag.Title()),
		Album:  strings.TrimSpace(tag.Album()),
		Artist: strings.TrimSpace(tag.Artist()),
	}
	trackTagsCache[p] = tags
	return tags, nil
}

// presetTrack returns the preset and file name of a track URI served from the
// embedded music/presets folder, e.g. http://host:8080/music/presets/7/a.mp3.
// ok is false for any other URI.
func presetTrack(uri string) (preset, filename string, ok bool) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", false
	}
	rest, found := strings.CutPrefix(u.Path, "/music/presets/")
	if !found {
		return "", "", false
	}
	preset, filename, found = strings.Cut(rest, "/")
	if !found || preset == "" || filename == "" || strings.Contains(filename, "/") {
		return "", "", false
	}
	return preset, filename, true
}

// newTrackResponse describes the mp3 file filename of preset playing on
// speaker. preset is empty if the track is not an embedded preset file.
func newTrackResponse(speaker, preset, filename string) TrackResponse {
	resp := TrackResponse{
		Preset:   preset,
		Speaker:  speaker,
		Filename: filename,
		Title:    strings.TrimSuffix(filename, path.Ext(filename)),
	}
	if preset == "" {
		return resp
	}

	tags, err := getTrackTags(path.Join("music/presets", preset, filename))
	if err != nil {
		log.Printf("Warning: Failed to read ID3 tags of preset %s %s: %v", preset, filename, err)
	}
	if tags.Title != "" {
		resp.Title = tags.Title
	}
	resp.Album = tags.Album
	resp.Artist = tags.Artist

	art, name, err := trackArtwork(preset, filename)
	if err != nil {
		log.Printf("Warning: Failed to load artwork of preset %s %s: %v", preset, filename, err)
	} else if art != nil {
		resp.ArtworkURL = artworkPath(preset, name)
		resp.ArtworkETag = art.ETag
		timeout := int(artworkTimeout / time.Second)
		resp.ArtworkTimeoutSeconds = &timeout
	}
	return resp
}

// currentTrackResponse describes the track currently loaded on speaker.
func currentTrackResponse(s *sonos.Sonos, speaker string) (TrackResponse, error) {
	info, err := s.GetPositionInfo(0)
	if err != nil {
		return TrackResponse{}, err
	}
	if preset, filename, ok := presetTrack(info.TrackURI); ok {
		return newTrackResponse(speaker, preset, filename), nil
	}
	return newTrackResponse(speaker, "", trackFilename(info.TrackURI)), nil
}

// writeTrackResponse writes resp as the JSON response body.
func writeTrackResponse(w http.ResponseWriter, resp TrackResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Leave characters such as & unescaped for the CardPuter's simple parser
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(resp)
}

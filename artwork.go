package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"
)

// artworkFilename is the optional artwork image in each preset folder.
const artworkFilename = "artwork.jpg"

// artworkSize is the maximum width and height of artwork served to the
// CardPuter, whose screen is 240x135. Configured by the -artwork-size flag.
var artworkSize = 135

// artworkTimeout is how long the CardPuter shows artwork before turning off
// the screen, zero to keep it on. Configured by the -artwork-timeout flag.
var artworkTimeout = 30 * time.Second

// Artwork is a preset's artwork scaled for the CardPuter screen.
type Artwork struct {
	Data []byte
	// ETag is the SHA-256 checksum of Data, hex encoded.
	ETag string
}

var (
	artworkMu    sync.Mutex
	artworkCache = make(map[string]*Artwork)
)

// artworkPath returns the URL path the artwork for preset is served from.
func artworkPath(preset string) string {
	return "/sonos/artwork/" + preset
}

// getArtwork returns the scaled artwork for preset, or nil if the preset has
// no artwork. Embedded artwork never changes so results are cached.
func getArtwork(preset string) (*Artwork, error) {
	artworkMu.Lock()
	defer artworkMu.Unlock()
	if art, ok := artworkCache[preset]; ok {
		return art, nil
	}

	src, err := musicFS.ReadFile(fmt.Sprintf("music/presets/%s/%s", preset, artworkFilename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	data, err := scaleJPEG(src, artworkSize)
	if err != nil {
		return nil, fmt.Errorf("preset %s artwork: %w", preset, err)
	}
	sum := sha256.Sum256(data)
	art := &Artwork{Data: data, ETag: hex.EncodeToString(sum[:])}
	log.Printf("Loaded artwork for preset %s: %d bytes scaled to %d bytes, etag %s", preset, len(src), len(data), art.ETag[:12])
	artworkCache[preset] = art
	return art, nil
}

// scaleJPEG decodes src and re-encodes it as a baseline JPEG no larger than
// size x size, preserving the aspect ratio.
func scaleJPEG(src []byte, size int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > size || h > size {
		if w >= h {
			w, h = size, max(1, h*size/b.Dx())
		} else {
			w, h = max(1, w*size/b.Dy()), size
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}

// artworkHandler serves preset artwork at /sonos/artwork/{preset}.
func artworkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	preset := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/sonos/artwork/"))
	if !validPresetName(preset) {
		http.Error(w, "Invalid preset", http.StatusBadRequest)
		return
	}
	art, err := getArtwork(preset)
	if err != nil {
		log.Printf("Failed to load artwork: %v", err)
		http.Error(w, "Failed to load artwork", http.StatusInternalServerError)
		return
	}
	if art == nil {
		http.NotFound(w, r)
		return
	}

	etag := `"` + art.ETag + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprint(len(art.Data)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		w.Write(art.Data)
	}
}

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
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jeffmccune/sonoserve/internal/mp3tag"
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

// Artwork is preset or track artwork scaled for the CardPuter screen.
type Artwork struct {
	Data []byte
	// ETag is the SHA-256 checksum of Data, hex encoded.
	ETag string
}

var (
	artworkMu sync.Mutex
	// artworkCache maps the embedded path of an artwork file to the scaled
	// artwork, nil if the file does not exist.
	artworkCache = make(map[string]*Artwork)
)

// artworkPath returns the URL path the artwork file name in preset is served
// from. The preset artwork.jpg is served from /sonos/artwork/{preset}, track
// artwork from /sonos/artwork/{preset}/{name}.
func artworkPath(preset, name string) string {
	if name == artworkFilename {
		return "/sonos/artwork/" + preset
	}
	return "/sonos/artwork/" + preset + "/" + url.PathEscape(name)
}

// getArtwork returns the scaled preset artwork.jpg, or nil if the preset has
// no artwork.
func getArtwork(preset string) (*Artwork, error) {
	return getArtworkFile(preset, artworkFilename)
}

// getArtworkFile returns the scaled artwork file name in preset, or nil if it
// does not exist. Embedded artwork never changes so results are cached.
func getArtworkFile(preset, name string) (*Artwork, error) {
	p := path.Join("music/presets", preset, name)
	artworkMu.Lock()
	defer artworkMu.Unlock()
	if art, ok := artworkCache[p]; ok {
		return art, nil
	}

	src, err := musicFS.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			artworkCache[p] = nil
			return nil, nil
		}
		return nil, err
	}
	data, err := scaleJPEG(src, artworkSize)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	sum := sha256.Sum256(data)
	art := &Artwork{Data: data, ETag: hex.EncodeToString(sum[:])}
	log.Printf("Loaded artwork %s: %d bytes scaled to %d bytes, etag %s", p, len(src), len(data), art.ETag[:12])
	artworkCache[p] = art
	return art, nil
}

// trackArtwork returns the artwork of the mp3 file name in preset and its
// file name: the track's own JPEG exported by export-tags, otherwise the
// preset artwork.jpg. It returns nil if there is neither.
func trackArtwork(preset, name string) (*Artwork, string, error) {
	fsys, err := fs.Sub(musicFS, path.Join("music/presets", preset))
	if err != nil {
		return nil, "", err
	}
	tagsFile, err := mp3tag.TagsFile(fsys, name)
	if err != nil {
		return nil, "", err
	}
	artworkFile, err := mp3tag.ArtworkFile(fsys, name, tagsFile)
	if err != nil || artworkFile == "" {
		return nil, "", err
	}
	art, err := getArtworkFile(preset, artworkFile)
	return art, artworkFile, err
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

// artworkHandler serves preset artwork.jpg at /sonos/artwork/{preset} and
// track artwork at /sonos/artwork/{preset}/{name}.jpg.
func artworkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	preset, name, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/sonos/artwork/"), "/")
	preset = strings.ToLower(preset)
	if name == "" {
		name = artworkFilename
	}
	if !validPresetName(preset) {
		http.Error(w, "Invalid preset", http.StatusBadRequest)
		return
	}
	if strings.Contains(name, "/") || !strings.HasSuffix(name, mp3tag.ArtworkExt) {
		http.Error(w, "Invalid artwork", http.StatusBadRequest)
		return
	}
	art, err := getArtworkFile(preset, name)
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

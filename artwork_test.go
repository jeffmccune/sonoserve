package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestScaleJPEG(t *testing.T) {
	cases := []struct{ w, h, wantW, wantH int }{
		{1200, 1200, 135, 135},
		{1200, 600, 135, 67},
		{600, 1200, 67, 135},
		{100, 50, 100, 50},
	}
	for _, c := range cases {
		out, err := scaleJPEG(testJPEG(t, c.w, c.h), 135)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != c.wantW || cfg.Height != c.wantH {
			t.Errorf("%dx%d scaled to %dx%d, want %dx%d", c.w, c.h, cfg.Width, cfg.Height, c.wantW, c.wantH)
		}
	}
}

func TestArtworkHandler(t *testing.T) {
	artworkMu.Lock()
	artworkCache["zz"] = &Artwork{Data: []byte("jpeg"), ETag: "abc123"}
	artworkMu.Unlock()
	defer func() {
		artworkMu.Lock()
		delete(artworkCache, "zz")
		artworkMu.Unlock()
	}()

	rr := httptest.NewRecorder()
	artworkHandler(rr, httptest.NewRequest("GET", "/sonos/artwork/ZZ", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "jpeg" {
		t.Fatalf("got %d %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("ETag"); got != `"abc123"` {
		t.Errorf("ETag = %s", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %s", got)
	}

	req := httptest.NewRequest("GET", "/sonos/artwork/zz", nil)
	req.Header.Set("If-None-Match", `"abc123"`)
	rr = httptest.NewRecorder()
	artworkHandler(rr, req)
	if rr.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: got %d, want 304", rr.Code)
	}

	rr = httptest.NewRecorder()
	artworkHandler(rr, httptest.NewRequest("GET", "/sonos/artwork/nosuchpreset", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("missing artwork: got %d, want 404", rr.Code)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestEmbeddedProfiles(t *testing.T) {
	all, err := loadProfiles(profilesFS)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"1": "Clark", "2": "Rylo", "3": "Jeff"}
	for id, name := range want {
		p := all[id]
		if p == nil {
			t.Fatalf("profile %s missing", id)
		}
		if p.Name != name {
			t.Errorf("profile %s: got name %q, want %q", id, p.Name, name)
		}
		// Every profile has a local image, used directly or as the fallback
		// when a remote image cannot be fetched
		if _, err := profilesFS.ReadFile("profiles/" + id + "/" + defaultAvatarFilename); err != nil {
			t.Errorf("profile %s: %v", id, err)
		}
	}
	if got := sortedProfileIDs(all); strings.Join(got, ",") != "1,2,3" {
		t.Errorf("sorted ids: got %v", got)
	}
}

func TestLoadProfilesInvalid(t *testing.T) {
	cases := map[string]string{
		"no name":        "speaker: x\n",
		"unknown field":  "name: a\nbogus: 1\n",
		"bad color":      "name: a\ncolor: red\n",
		"bad volume":     "name: a\nmax_volume: 101\n",
		"bad action":     "name: a\nkeys:\n  x: explode\n",
		"long key":       "name: a\nkeys:\n  xy: play\n",
		"image path":     "name: a\nimage: ../2/avatar.jpg\n",
		"invalid preset": "name: a\nkeys:\n  x: preset:A.B\n",
	}
	for name, yml := range cases {
		fsys := fstest.MapFS{"profiles/1/profile.yaml": {Data: []byte(yml)}}
		if _, err := loadProfiles(fsys); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestProfileAction(t *testing.T) {
	p := &Profile{Keys: map[string]string{"m": "mute", "z": "none", "[": "play-pause", "!": "preset:9"}}
	cases := map[string]string{
		"a": "preset:a",
		"A": "preset:a",
		"7": "preset:7",
		"m": "mute",
		"z": "",
		"[": "play-pause",
		"]": "play",
		",": "previous",
		"/": "next",
		";": "volume-up",
		".": "volume-down",
		"!": "preset:9",
		"?": "",
		"":  "",
	}
	for key, want := range cases {
		got, ok := p.Action(key)
		if got != want || ok != (want != "") {
			t.Errorf("key %q: got %q %v, want %q", key, got, ok, want)
		}
	}
	keys := p.KeyMap()
	if keys["m"] != "mute" || keys["!"] != "preset:9" || keys["a"] != "preset:a" {
		t.Errorf("key map: %v", keys)
	}
	if _, ok := keys["z"]; ok {
		t.Errorf("key map: z should do nothing: %v", keys)
	}
}

func TestRGB565(t *testing.T) {
	cases := map[uint32]int{0xffffff: 0xffff, 0xff0000: 0xf800, 0x00ff00: 0x07e0, 0x0000ff: 0x001f, 0: 0}
	for in, want := range cases {
		if got := rgb565(in); got != want {
			t.Errorf("rgb565(%06x): got %04x, want %04x", in, got, want)
		}
	}
}

func TestProfilesHandler(t *testing.T) {
	get := func(target string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		profilesHandler(rr, httptest.NewRequest("GET", target, nil))
		return rr
	}

	rr := get("/profiles/")
	var list []ProfileSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil || len(list) < 3 || list[0].Name != "Clark" {
		t.Fatalf("list: %d %s %v", rr.Code, rr.Body.String(), err)
	}

	rr = get("/profiles/1")
	if rr.Code != http.StatusOK {
		t.Fatalf("profile: %d %s", rr.Code, rr.Body.String())
	}
	var p ProfileResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Name != "Clark" || p.AvatarURL != "/profiles/1/avatar" || p.AvatarETag == "" || p.Keys["a"] != "preset:a" {
		t.Errorf("profile: %+v", p)
	}
	// The CardPuter parser reads the first occurrence of each key, so the
	// flat fields must come before the key map
	if body := rr.Body.String(); strings.Index(body, `"keys"`) < strings.Index(body, `"avatar_etag"`) {
		t.Errorf("keys must come last: %s", body)
	}

	rr = get("/profiles/1/avatar")
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("avatar: %d %v", rr.Code, rr.Header())
	}
	img, _, err := image.Decode(bytes.NewReader(rr.Body.Bytes()))
	if err != nil || img.Bounds().Dx() > avatarSize || img.Bounds().Dy() > avatarSize {
		t.Errorf("avatar: %v %v", err, img)
	}
	req := httptest.NewRequest("GET", "/profiles/1/avatar", nil)
	req.Header.Set("If-None-Match", rr.Header().Get("ETag"))
	rr = httptest.NewRecorder()
	profilesHandler(rr, req)
	if rr.Code != http.StatusNotModified {
		t.Errorf("avatar If-None-Match: got %d", rr.Code)
	}

	rr = get("/profiles/1/profile.yaml")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "name: Clark") {
		t.Errorf("profile.yaml: %d %s", rr.Code, rr.Body.String())
	}

	if rr = get("/profiles/99"); rr.Code != http.StatusNotFound {
		t.Errorf("missing profile: got %d", rr.Code)
	}
	if rr = get("/profiles/1/key"); rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET key: got %d", rr.Code)
	}
}

func TestProfileKeyNoAction(t *testing.T) {
	rr := httptest.NewRecorder()
	profilesHandler(rr, httptest.NewRequest("POST", "/profiles/1/key", strings.NewReader(`{"key":"?"}`)))
	if rr.Code != http.StatusNoContent {
		t.Errorf("got %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	profilesHandler(rr, httptest.NewRequest("POST", "/profiles/1/key", strings.NewReader(`not json`)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid body: got %d", rr.Code)
	}
}

func TestProfileKeyDispatch(t *testing.T) {
	// The speaker is not in the cache, so the action handler responds 404
	// naming the speaker it was asked to control: the device setting, else
	// the profile speaker.
	for body, want := range map[string]string{
		`{"key":";","speaker":"Test Device Speaker"}`: "Test Device Speaker",
		`{"key":";"}`: "Kids Room",
	} {
		rr := httptest.NewRecorder()
		profilesHandler(rr, httptest.NewRequest("POST", "/profiles/1/key", strings.NewReader(body)))
		if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), want) {
			t.Errorf("%s: got %d %s", body, rr.Code, rr.Body.String())
		}
	}
	found := false
	for _, name := range scheduledSpeakers() {
		found = found || name == "Test Device Speaker"
	}
	if !found {
		t.Errorf("scheduled speakers should include the device speaker: %v", scheduledSpeakers())
	}
}

func TestVolumeOptionsLimit(t *testing.T) {
	day := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	if got := (volumeOptions{}).Limit(day); got != schedule.DayMaxVolume {
		t.Errorf("no max: got %d", got)
	}
	if got := (volumeOptions{Max: 5}).Limit(day); got != 5 {
		t.Errorf("max 5: got %d", got)
	}
	if got := (volumeOptions{Max: 100}).Limit(day); got != schedule.DayMaxVolume {
		t.Errorf("max 100: got %d", got)
	}
}

func TestRemoteAvatar(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 400, 200)))
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/u.png" {
			http.NotFound(w, r)
			return
		}
		w.Write(buf.Bytes())
	}))
	defer srv.Close()
	saved := avatarClient
	avatarClient = srv.Client()
	avatarClient.Timeout = 5 * time.Second
	defer func() { avatarClient = saved }()

	p := &Profile{ID: "test-remote", Image: srv.URL + "/u.png"}
	art, err := getAvatar(t.Context(), p)
	if err != nil || art == nil {
		t.Fatalf("remote png: %v %v", art, err)
	}
	img, _, err := image.Decode(bytes.NewReader(art.Data))
	if err != nil || img.Bounds().Dx() != avatarSize || img.Bounds().Dy() != avatarSize/2 {
		t.Errorf("remote png scaled: %v %v", err, img.Bounds())
	}

	// A remote image which cannot be fetched falls back to avatar.jpg
	p = &Profile{ID: "1", Image: srv.URL + "/missing.png"}
	avatarMu.Lock()
	delete(avatarCache, "1")
	avatarMu.Unlock()
	art, err = getAvatar(t.Context(), p)
	if err != nil || art == nil {
		t.Fatalf("fallback: %v %v", art, err)
	}
	avatarMu.Lock()
	delete(avatarCache, "1")
	avatarMu.Unlock()
}

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "image/gif"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Profiles live in profiles/{id}/profile.yaml, one per person, e.g. Clark is
// 1, Rylo is 2, Jeff is 3. Every CardPuter runs the same firmware and stores
// only its profile number. It fetches the profile from the server at startup
// and sends each key press to the server, which decides what the key does, so
// behavior changes by deploying a new server rather than reprogramming each
// device.
//
//go:embed all:profiles
var profilesFS embed.FS

// profileFilename is the profile settings file in each profile folder.
const profileFilename = "profile.yaml"

// defaultAvatarFilename is the image in the profile folder used when the
// profile image is not set or a remote image cannot be fetched.
const defaultAvatarFilename = "avatar.jpg"

// avatarSize is the maximum width and height of profile images served to the
// CardPuter.
var avatarSize = 96

// remoteAvatarTTL is how long a profile image fetched from an http(s) URL is
// cached before it is fetched again.
var remoteAvatarTTL = time.Hour

// Profile is the profile.yaml settings of a person.
type Profile struct {
	// ID is the name of the profile folder, e.g. "1".
	ID string `yaml:"-"`
	// Name is shown on the CardPuter ready screen.
	Name string `yaml:"name"`
	// Speaker is the Sonos speaker the profile controls. A speaker saved on
	// the device takes precedence, empty uses -default-speaker.
	Speaker string `yaml:"speaker"`
	// Image is a file name in the profile folder or an http(s) URL.
	Image string `yaml:"image"`
	// Color is the #RRGGBB accent color of the ready screen.
	Color string `yaml:"color"`
	// DefaultVolume is the volume a preset starts at, 0 leaves it unchanged.
	DefaultVolume int `yaml:"default_volume"`
	// MaxVolume is the volume limit, 0 for only the time of day schedule.
	MaxVolume int `yaml:"max_volume"`
	// Help is shown on the ready screen, one entry per line.
	Help []string `yaml:"help"`
	// Keys maps a key to an action, overriding defaultKeyAction.
	Keys map[string]string `yaml:"keys"`
}

// controlActions are the actions other than presets a key may perform, and
// the handler which performs each.
var controlActions = map[string]http.HandlerFunc{
	"play":        playHandler,
	"pause":       pauseHandler,
	"play-pause":  playPauseHandler,
	"next":        nextTrackHandler,
	"previous":    previousTrackHandler,
	"volume-up":   volumeUpHandler,
	"volume-down": volumeDownHandler,
	"mute":        muteHandler,
}

// noAction is the key action which does nothing, used to disable a default.
const noAction = "none"

// defaultKeyAction returns the action of key when the profile does not
// override it, matching the original CardPuter firmware: 0-9 and a-z play
// the preset of the same name.
func defaultKeyAction(key string) string {
	switch key {
	case "[":
		return "pause"
	case "]":
		return "play"
	case ",":
		return "previous"
	case "/":
		return "next"
	case ";":
		return "volume-up"
	case ".":
		return "volume-down"
	}
	if len(key) == 1 && validPresetName(key) {
		return "preset:" + key
	}
	return ""
}

// validAction reports whether action is a known key action.
func validAction(action string) bool {
	if action == noAction {
		return true
	}
	if preset, ok := strings.CutPrefix(action, "preset:"); ok {
		return validPresetName(preset)
	}
	_, ok := controlActions[action]
	return ok
}

// Action returns the action of key, false if the key does nothing.
func (p *Profile) Action(key string) (string, bool) {
	if len(key) == 1 && key[0] >= 'A' && key[0] <= 'Z' {
		key = strings.ToLower(key)
	}
	action, ok := p.Keys[key]
	if !ok {
		action = defaultKeyAction(key)
	}
	if action == "" || action == noAction {
		return "", false
	}
	return action, true
}

// KeyMap returns the action of every key that does something.
func (p *Profile) KeyMap() map[string]string {
	keys := map[string]string{}
	for _, k := range strings.Split("abcdefghijklmnopqrstuvwxyz0123456789[],/;.", "") {
		keys[k] = defaultKeyAction(k)
	}
	for k, v := range p.Keys {
		keys[k] = v
	}
	for k, v := range keys {
		if v == "" || v == noAction {
			delete(keys, k)
		}
	}
	return keys
}

// validate checks the profile settings.
func (p *Profile) validate() error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if _, err := parseColor(p.Color); err != nil {
		return err
	}
	for name, v := range map[string]int{"default_volume": p.DefaultVolume, "max_volume": p.MaxVolume} {
		if v < 0 || v > 100 {
			return fmt.Errorf("%s %d must be between 0 and 100", name, v)
		}
	}
	for key, action := range p.Keys {
		if len(key) != 1 {
			return fmt.Errorf("key %q must be a single character", key)
		}
		if !validAction(action) {
			return fmt.Errorf("key %q has unknown action %q", key, action)
		}
	}
	if p.Image != "" && !isRemoteImage(p.Image) && (strings.Contains(p.Image, "/") || strings.HasPrefix(p.Image, ".")) {
		return fmt.Errorf("image %q must be a file name in the profile folder or an http(s) URL", p.Image)
	}
	return nil
}

// parseColor parses a #RRGGBB color, black if empty.
func parseColor(s string) (uint32, error) {
	if s == "" {
		return 0, nil
	}
	hexColor, ok := strings.CutPrefix(s, "#")
	if !ok || len(hexColor) != 6 {
		return 0, fmt.Errorf("color %q must be #RRGGBB", s)
	}
	v, err := strconv.ParseUint(hexColor, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("color %q must be #RRGGBB", s)
	}
	return uint32(v), nil
}

// rgb565 converts a 24 bit RGB color to the 16 bit color of the CardPuter
// display.
func rgb565(c uint32) int {
	r, g, b := (c>>16)&0xff, (c>>8)&0xff, c&0xff
	return int((r>>3)<<11 | (g>>2)<<5 | b>>3)
}

func isRemoteImage(image string) bool {
	return strings.HasPrefix(image, "https://") || strings.HasPrefix(image, "http://")
}

// loadProfiles reads every profiles/{id}/profile.yaml in fsys.
func loadProfiles(fsys fs.FS) (map[string]*Profile, error) {
	entries, err := fs.ReadDir(fsys, "profiles")
	if err != nil {
		return nil, err
	}
	profiles := make(map[string]*Profile)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if !validPresetName(id) {
			return nil, fmt.Errorf("profile folder %q must be lower case letters and digits", id)
		}
		data, err := fs.ReadFile(fsys, path.Join("profiles", id, profileFilename))
		if err != nil {
			return nil, fmt.Errorf("profile %s: %w", id, err)
		}
		p := &Profile{ID: id}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(p); err != nil {
			return nil, fmt.Errorf("profile %s: %w", id, err)
		}
		if err := p.validate(); err != nil {
			return nil, fmt.Errorf("profile %s: %w", id, err)
		}
		profiles[id] = p
	}
	return profiles, nil
}

// profiles holds the embedded profiles, loaded once on first use.
var profiles = sync.OnceValues(func() (map[string]*Profile, error) {
	return loadProfiles(profilesFS)
})

// getProfile returns the profile with id, nil if there is none.
func getProfile(id string) (*Profile, error) {
	all, err := profiles()
	if err != nil {
		return nil, err
	}
	return all[id], nil
}

// sortedProfileIDs returns the ids of all profiles, numbers in numeric order.
func sortedProfileIDs(all map[string]*Profile) []string {
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, aerr := strconv.Atoi(ids[i])
		b, berr := strconv.Atoi(ids[j])
		if aerr == nil && berr == nil {
			return a < b
		}
		return ids[i] < ids[j]
	})
	return ids
}

// ProfileResponse is the response body of GET /profiles/{id}. The fields the
// CardPuter reads come first and are flat for its simple JSON parser.
type ProfileResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Speaker       string `json:"speaker"`
	Color         string `json:"color"`
	ColorRGB565   int    `json:"color_rgb565"`
	DefaultVolume int    `json:"default_volume"`
	MaxVolume     int    `json:"max_volume"`
	// Help is the ready screen help, one line per newline.
	Help string `json:"help"`
	// AvatarURL is the URL path of the profile image scaled for the
	// CardPuter, empty if the profile has none.
	AvatarURL  string `json:"avatar_url,omitempty"`
	AvatarETag string `json:"avatar_etag,omitempty"`
	// Image is the image setting of profile.yaml.
	Image string `json:"image,omitempty"`
	// Keys is the action of every key that does something.
	Keys map[string]string `json:"keys"`
}

func newProfileResponse(ctx context.Context, p *Profile) ProfileResponse {
	color, _ := parseColor(p.Color)
	resp := ProfileResponse{
		ID:            p.ID,
		Name:          p.Name,
		Speaker:       p.Speaker,
		Color:         p.Color,
		ColorRGB565:   rgb565(color),
		DefaultVolume: p.DefaultVolume,
		MaxVolume:     p.MaxVolume,
		Help:          strings.Join(p.Help, "\n"),
		Image:         p.Image,
		Keys:          p.KeyMap(),
	}
	if resp.Speaker == "" {
		resp.Speaker = defaultSpeaker
	}
	if art, err := getAvatar(ctx, p); err != nil {
		log.Printf("Warning: Failed to load avatar of profile %s: %v", p.ID, err)
	} else if art != nil {
		resp.AvatarURL = "/profiles/" + p.ID + "/avatar"
		resp.AvatarETag = art.ETag
	}
	return resp
}

type cachedAvatar struct {
	art     *Artwork
	expires time.Time
}

var (
	avatarMu    sync.Mutex
	avatarCache = make(map[string]cachedAvatar)
	// avatarClient fetches remote profile images.
	avatarClient = &http.Client{Timeout: 10 * time.Second}
)

// maxRemoteImageBytes limits the size of a remote profile image.
const maxRemoteImageBytes = 5 << 20

// getAvatar returns the profile image scaled for the CardPuter, nil if the
// profile has none. A remote image is cached for remoteAvatarTTL and falls
// back to avatar.jpg in the profile folder if it cannot be fetched.
func getAvatar(ctx context.Context, p *Profile) (*Artwork, error) {
	avatarMu.Lock()
	defer avatarMu.Unlock()
	if c, ok := avatarCache[p.ID]; ok && time.Now().Before(c.expires) {
		return c.art, nil
	}

	var src []byte
	var err error
	ttl := 100 * 365 * 24 * time.Hour // embedded images never change
	image := p.Image
	if image == "" {
		image = defaultAvatarFilename
	}
	if isRemoteImage(image) {
		ttl = remoteAvatarTTL
		src, err = fetchImage(ctx, image)
		if err != nil {
			log.Printf("Warning: Failed to fetch image of profile %s, using %s: %v", p.ID, defaultAvatarFilename, err)
			// Try the remote image again soon
			ttl = time.Minute
			image = defaultAvatarFilename
		}
	}
	if src == nil {
		src, err = profilesFS.ReadFile(path.Join("profiles", p.ID, image))
		if errors.Is(err, fs.ErrNotExist) {
			avatarCache[p.ID] = cachedAvatar{nil, time.Now().Add(ttl)}
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}

	data, err := scaleJPEG(src, avatarSize)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", image, err)
	}
	sum := sha256.Sum256(data)
	art := &Artwork{Data: data, ETag: hex.EncodeToString(sum[:])}
	avatarCache[p.ID] = cachedAvatar{art, time.Now().Add(ttl)}
	log.Printf("Loaded image of profile %s from %s: %d bytes, etag %s", p.ID, image, len(data), art.ETag[:12])
	return art, nil
}

// fetchImage fetches an image from an http(s) URL.
func fetchImage(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, avatarClient.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := avatarClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRemoteImageBytes {
		return nil, fmt.Errorf("GET %s: image larger than %d bytes", url, maxRemoteImageBytes)
	}
	return data, nil
}

// profilesHandler serves:
//
//	GET  /profiles/                    list of profiles
//	GET  /profiles/{id}                the profile as JSON
//	GET  /profiles/{id}/profile.yaml   the profile settings file
//	GET  /profiles/{id}/avatar         the profile image scaled for the CardPuter
//	POST /profiles/{id}/key            perform the action of a key press
func profilesHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/profiles"), "/")
	if rest == "" {
		profileListHandler(w, r)
		return
	}
	id, sub, _ := strings.Cut(rest, "/")
	p, err := getProfile(id)
	if err != nil {
		log.Printf("Failed to load profiles: %v", err)
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, fmt.Sprintf("Profile '%s' not found", id), http.StatusNotFound)
		return
	}

	switch sub {
	case "":
		if !allowMethods(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, newProfileResponse(r.Context(), p))
	case profileFilename:
		if !allowMethods(w, r, http.MethodGet) {
			return
		}
		data, err := profilesFS.ReadFile(path.Join("profiles", p.ID, profileFilename))
		if err != nil {
			http.Error(w, "Failed to read profile", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Write(data)
	case "avatar":
		if !allowMethods(w, r, http.MethodGet, http.MethodHead) {
			return
		}
		avatarHandler(w, r, p)
	case "key":
		if !allowMethods(w, r, http.MethodPost) {
			return
		}
		profileKeyHandler(w, r, p)
	default:
		http.NotFound(w, r)
	}
}

func allowMethods(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, m := range methods {
		if r.Method == m {
			return true
		}
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	// Leave characters such as & unescaped for the CardPuter's simple parser
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

// ProfileSummary is an entry of GET /profiles/.
type ProfileSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func profileListHandler(w http.ResponseWriter, r *http.Request) {
	if !allowMethods(w, r, http.MethodGet) {
		return
	}
	all, err := profiles()
	if err != nil {
		log.Printf("Failed to load profiles: %v", err)
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}
	list := []ProfileSummary{}
	for _, id := range sortedProfileIDs(all) {
		list = append(list, ProfileSummary{ID: id, Name: all[id].Name})
	}
	writeJSON(w, list)
}

func avatarHandler(w http.ResponseWriter, r *http.Request, p *Profile) {
	art, err := getAvatar(r.Context(), p)
	if err != nil {
		log.Printf("Failed to load avatar of profile %s: %v", p.ID, err)
		http.Error(w, "Failed to load profile image", http.StatusInternalServerError)
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

// KeyRequest is the request body of POST /profiles/{id}/key. Speaker,
// DefaultVolume, and MaxVolume are the device settings, which take precedence
// over the profile when set.
type KeyRequest struct {
	Key           string `json:"key"`
	Speaker       string `json:"speaker,omitempty"`
	DefaultVolume int    `json:"default_volume,omitempty"`
	MaxVolume     int    `json:"max_volume,omitempty"`
}

// profileKeyHandler performs the action of a key pressed on a CardPuter. The
// response is the response of the action, or 204 No Content if the key does
// nothing.
func profileKeyHandler(w http.ResponseWriter, r *http.Request, p *Profile) {
	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	action, ok := p.Action(req.Key)
	if !ok {
		log.Printf("Profile %s: key %q does nothing", p.ID, req.Key)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	speaker := firstNonEmpty(req.Speaker, p.Speaker, defaultSpeaker)
	rememberSpeaker(speaker)
	opts := volumeOptions{
		Default: firstPositive(req.DefaultVolume, p.DefaultVolume),
		Max:     firstPositive(req.MaxVolume, p.MaxVolume),
	}
	log.Printf("Profile %s (%s): key %q is %s on %s", p.ID, p.Name, req.Key, action, speaker)

	handler, actionPath := actionHandler(action)
	if handler == nil {
		http.Error(w, fmt.Sprintf("Unknown action %q", action), http.StatusInternalServerError)
		return
	}
	body, _ := json.Marshal(map[string]string{"speaker": speaker})
	r2 := r.Clone(withVolumeOptions(r.Context(), opts))
	r2.Method = http.MethodPost
	r2.URL.Path = actionPath
	r2.Body = io.NopCloser(bytes.NewReader(body))
	r2.ContentLength = int64(len(body))
	handler(w, r2)
}

// actionHandler returns the handler and URL path which perform action.
func actionHandler(action string) (http.HandlerFunc, string) {
	if preset, ok := strings.CutPrefix(action, "preset:"); ok {
		return presetHandler, "/sonos/preset/" + preset
	}
	if h, ok := controlActions[action]; ok {
		return h, "/sonos/" + action
	}
	return nil, ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

// volumeOptions are the volume settings of the profile or device making a
// request.
type volumeOptions struct {
	// Default is the volume a preset starts at, 0 leaves it unchanged.
	Default int
	// Max is the volume limit, 0 for only the time of day schedule.
	Max int
}

type volumeOptionsKey struct{}

func withVolumeOptions(ctx context.Context, o volumeOptions) context.Context {
	return context.WithValue(ctx, volumeOptionsKey{}, o)
}

func volumeOptionsFrom(ctx context.Context) volumeOptions {
	o, _ := ctx.Value(volumeOptionsKey{}).(volumeOptions)
	return o
}

// Limit returns the maximum volume at now: the lower of the time of day
// schedule and Max.
func (o volumeOptions) Limit(now time.Time) int {
	limit := schedule.MaxVolume(now)
	if o.Max > 0 && o.Max < limit {
		limit = o.Max
	}
	return limit
}

var (
	seenSpeakersMu sync.Mutex
	// seenSpeakers are the speakers controlled through profiles, including
	// speakers saved in device settings.
	seenSpeakers = make(map[string]bool)
)

func rememberSpeaker(name string) {
	seenSpeakersMu.Lock()
	defer seenSpeakersMu.Unlock()
	seenSpeakers[name] = true
}

// scheduledSpeakers returns the speakers the time of day schedule applies to:
// the default speaker, the speaker of each profile, and every speaker
// controlled through a profile since the server started.
func scheduledSpeakers() []string {
	set := map[string]bool{defaultSpeaker: true}
	if all, err := profiles(); err == nil {
		for _, p := range all {
			if p.Speaker != "" {
				set[p.Speaker] = true
			}
		}
	}
	seenSpeakersMu.Lock()
	for name := range seenSpeakers {
		set[name] = true
	}
	seenSpeakersMu.Unlock()
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

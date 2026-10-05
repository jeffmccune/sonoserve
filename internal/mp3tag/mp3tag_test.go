package mp3tag

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bogem/id3v2/v2"
)

func TestFileName(t *testing.T) {
	tests := map[string]string{
		"You're Welcome":                  "Youre Welcome",
		"Prologue:  Beauty and the Beast": "Prologue Beauty and the Beast",
		"Old McColl’s Farm":               "Old McColls Farm",
		"Wi$h Li$t":                       "Wi$h Li$t",
		"Tala's Deathbed":                 "Talas Deathbed",
	}
	for in, want := range tests {
		if got := FileName(in); got != want {
			t.Errorf("FileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTagsFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"01-Same.yaml", "02-Other Name.yaml", "03-A.yaml", "03-B.yaml", "10-Ten.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		mp3, want string
		wantErr   bool
	}{
		{mp3: "01-Same.mp3", want: "01-Same.yaml"},
		{mp3: "02-Renamed.mp3", want: "02-Other Name.yaml"},
		{mp3: "03-C.mp3", wantErr: true},
		{mp3: "1-One.mp3", want: ""},
		{mp3: "04-None.mp3", want: ""},
		{mp3: "No Number.mp3", want: ""},
	}
	for _, tt := range tests {
		got, err := TagsFile(filepath.Join(dir, tt.mp3))
		if (err != nil) != tt.wantErr {
			t.Errorf("TagsFile(%q) error = %v, wantErr %v", tt.mp3, err, tt.wantErr)
			continue
		}
		if tt.want != "" {
			tt.want = filepath.Join(dir, tt.want)
		}
		if got != tt.want {
			t.Errorf("TagsFile(%q) = %q, want %q", tt.mp3, got, tt.want)
		}
	}
}

func TestReadTagsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "01-Typo.yaml")
	if err := os.WriteFile(path, []byte("titel: Typo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTags(path); err == nil || !strings.Contains(err.Error(), "titel") {
		t.Errorf("ReadTags() error = %v, want unknown field titel", err)
	}
}

// TestRoundTrip checks tags survive YAML and an ID3v2.3 tag unchanged, which
// embed-tags relies on to skip files that are already up to date.
func TestRoundTrip(t *testing.T) {
	want := &Tags{
		Title:       "You're Welcome",
		Artist:      "Opetaia Foa’i",
		Album:       "Moana",
		Year:        2016,
		Track:       7,
		TrackCount:  13,
		Disc:        1,
		Compilation: true,
		Comment:     "A comment",
		Lyrics:      "Ō line one\nline two",
		SortArtist:  "Foa’i, Opetaia",
	}
	path := filepath.Join(t.TempDir(), "07-Youre Welcome.yaml")
	data, err := want.Marshal("header")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTags(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadTags() = %+v, want %+v", got, want)
	}

	tag := id3v2.NewEmptyTag()
	tag.SetVersion(3)
	got.Apply(tag)
	var buf bytes.Buffer
	if _, err := tag.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	parsed, err := id3v2.ParseReader(&buf, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version() != 3 {
		t.Errorf("version = %d, want 3", parsed.Version())
	}
	if !reflect.DeepEqual(parsed.AllFrames(), tag.AllFrames()) {
		t.Errorf("parsed frames = %+v, want %+v", parsed.AllFrames(), tag.AllFrames())
	}
	if got := parsed.GetTextFrame("TRCK").Text; got != "7/13" {
		t.Errorf("TRCK = %q, want 7/13", got)
	}
}

func TestIsProgressiveJPEG(t *testing.T) {
	baseline := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x04, 0x00, 0x00, 0xFF, 0xC0, 0x00, 0x02}
	progressive := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x04, 0x00, 0x00, 0xFF, 0xC2, 0x00, 0x02}
	if p, err := isProgressiveJPEG(baseline); err != nil || p {
		t.Errorf("baseline: got %v, %v", p, err)
	}
	if p, err := isProgressiveJPEG(progressive); err != nil || !p {
		t.Errorf("progressive: got %v, %v", p, err)
	}
	if _, err := isProgressiveJPEG([]byte("PNG")); err == nil {
		t.Error("non-JPEG: want error")
	}
}

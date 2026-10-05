// Package mp3tag reads and writes the ID3v2.3 tags of preset mp3 files.
//
// ID3v2.3 with ISO-8859-1 or UTF-16 text is the most widely supported
// combination, so every tag written by this package uses it.
package mp3tag

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bogem/id3v2/v2"
	"gopkg.in/yaml.v3"
)

// ArtworkFilename is the artwork image in each preset folder.
const ArtworkFilename = "artwork.jpg"

// TagsExt is the file extension of the YAML tags file of an mp3 file.
const TagsExt = ".yaml"

// pictureDescription is the description of the embedded front cover.
const pictureDescription = "Cover"

// language is the ISO-639-2 language of comment and lyrics frames.
const language = "eng"

// Tags is the metadata of one track, stored as YAML next to its mp3 file.
// Every field maps to one ID3v2.3 frame, noted in the comments. Empty fields
// are omitted from the tag.
type Tags struct {
	Title       string `yaml:"title,omitempty"`        // TIT2
	Artist      string `yaml:"artist,omitempty"`       // TPE1
	AlbumArtist string `yaml:"album_artist,omitempty"` // TPE2
	Album       string `yaml:"album,omitempty"`        // TALB
	Composer    string `yaml:"composer,omitempty"`     // TCOM
	Genre       string `yaml:"genre,omitempty"`        // TCON
	Grouping    string `yaml:"grouping,omitempty"`     // TIT1
	Year        int    `yaml:"year,omitempty"`         // TYER
	Track       int    `yaml:"track,omitempty"`        // TRCK track/track_count
	TrackCount  int    `yaml:"track_count,omitempty"`  // TRCK
	Disc        int    `yaml:"disc,omitempty"`         // TPOS disc/disc_count
	DiscCount   int    `yaml:"disc_count,omitempty"`   // TPOS
	BPM         int    `yaml:"bpm,omitempty"`          // TBPM
	Compilation bool   `yaml:"compilation,omitempty"`  // TCMP
	Comment     string `yaml:"comment,omitempty"`      // COMM
	Lyrics      string `yaml:"lyrics,omitempty"`       // USLT

	SortTitle       string `yaml:"sort_title,omitempty"`        // TSOT
	SortArtist      string `yaml:"sort_artist,omitempty"`       // TSOP
	SortAlbumArtist string `yaml:"sort_album_artist,omitempty"` // TSO2
	SortAlbum       string `yaml:"sort_album,omitempty"`        // TSOA
	SortComposer    string `yaml:"sort_composer,omitempty"`     // TSOC
}

// ReadTags reads a YAML tags file. Unknown fields are an error so typos in
// hand edited files are caught.
func ReadTags(path string) (*Tags, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var t Tags
	if err := dec.Decode(&t); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &t, nil
}

// Marshal returns t as YAML preceded by header as a comment.
func (t *Tags) Marshal(header string) ([]byte, error) {
	var buf bytes.Buffer
	for line := range strings.SplitSeq(strings.TrimSpace(header), "\n") {
		buf.WriteString("# " + line + "\n")
	}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(t); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

// Apply adds the frames of t to tag, which must be version 3.
func (t *Tags) Apply(tag *id3v2.Tag) {
	text := func(id, value string) {
		if value != "" {
			tag.AddFrame(id, id3v2.TextFrame{Encoding: Encoding(value), Text: value})
		}
	}
	number := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	position := func(n, count int) string {
		if n == 0 {
			return ""
		}
		if count == 0 {
			return strconv.Itoa(n)
		}
		return fmt.Sprintf("%d/%d", n, count)
	}

	text("TIT2", t.Title)
	text("TPE1", t.Artist)
	text("TPE2", t.AlbumArtist)
	text("TALB", t.Album)
	text("TCOM", t.Composer)
	text("TCON", t.Genre)
	text("TIT1", t.Grouping)
	text("TYER", number(t.Year))
	text("TRCK", position(t.Track, t.TrackCount))
	text("TPOS", position(t.Disc, t.DiscCount))
	text("TBPM", number(t.BPM))
	if t.Compilation {
		text("TCMP", "1")
	}
	if t.Comment != "" {
		tag.AddCommentFrame(id3v2.CommentFrame{
			Encoding: Encoding(t.Comment),
			Language: language,
			Text:     t.Comment,
		})
	}
	if t.Lyrics != "" {
		tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
			Encoding: Encoding(t.Lyrics),
			Language: language,
			Lyrics:   t.Lyrics,
		})
	}
	text("TSOT", t.SortTitle)
	text("TSOP", t.SortArtist)
	text("TSO2", t.SortAlbumArtist)
	text("TSOA", t.SortAlbum)
	text("TSOC", t.SortComposer)
}

// Encoding returns the ID3v2.3 text encoding for text: ISO-8859-1 when it is
// ASCII, otherwise UTF-16 with BOM. UTF-8 is not allowed in ID3v2.3.
func Encoding(text string) id3v2.Encoding {
	for _, r := range text {
		if r >= utf8.RuneSelf {
			return id3v2.EncodingUTF16
		}
	}
	return id3v2.EncodingISO
}

// ReadArtwork reads a JPEG image and checks it is baseline encoded, since
// some Sonos controllers fail to show progressive JPEG artwork.
func ReadArtwork(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	progressive, err := isProgressiveJPEG(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if progressive {
		return nil, fmt.Errorf("%s: progressive JPEG is not supported by Sonos, re-save it as baseline", path)
	}
	return data, nil
}

// isProgressiveJPEG scans the JPEG markers in data for the start of frame
// marker and reports whether it is progressive (SOF2).
func isProgressiveJPEG(data []byte) (bool, error) {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return false, errors.New("not a JPEG image")
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return false, errors.New("corrupt JPEG marker")
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF: // fill byte
			i++
			continue
		case marker == 0xC2:
			return true, nil
		case marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC:
			return false, nil
		}
		i += 2 + (int(data[i+2])<<8 | int(data[i+3]))
	}
	return false, errors.New("no JPEG start of frame")
}

// SetArtwork replaces every picture in tag with artwork as the front cover.
func SetArtwork(tag *id3v2.Tag, artwork []byte) {
	tag.DeleteFrames("APIC")
	tag.AddAttachedPicture(id3v2.PictureFrame{
		Encoding:    id3v2.EncodingISO,
		MimeType:    "image/jpeg",
		PictureType: id3v2.PTFrontCover,
		Description: pictureDescription,
		Picture:     artwork,
	})
}

// HasArtwork reports whether artwork is the only picture in tag.
func HasArtwork(tag *id3v2.Tag, artwork []byte) bool {
	pics := tag.GetFrames("APIC")
	if len(pics) != 1 {
		return false
	}
	pf, ok := pics[0].(id3v2.PictureFrame)
	return ok && pf.PictureType == id3v2.PTFrontCover && bytes.Equal(pf.Picture, artwork)
}

// id3v1Size is the size of an ID3v1 tag at the end of an mp3 file.
const id3v1Size = 128

// HasID3v1 reports whether file ends with an ID3v1 tag.
func HasID3v1(file string) (bool, error) {
	f, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() < id3v1Size {
		return false, nil
	}
	buf := make([]byte, 3)
	if _, err := f.ReadAt(buf, info.Size()-id3v1Size); err != nil {
		return false, err
	}
	return string(buf) == "TAG", nil
}

// StripID3v1 removes the ID3v1 tag from the end of file, if it has one.
// Players may fall back to stale ID3v1 tags, so authoritative writes remove
// them.
func StripID3v1(file string) error {
	ok, err := HasID3v1(file)
	if err != nil || !ok {
		return err
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	return os.Truncate(file, info.Size()-id3v1Size)
}

// FileName returns name with the characters go:embed rejects removed, so it
// can be used as a file name in the embedded music folder. Apostrophes are
// the most common, e.g. "You're Welcome" becomes "Youre Welcome".
func FileName(name string) string {
	const allowed = "!#$%&()+,-.=@[]^_{}~ "
	var b strings.Builder
	for _, r := range name {
		ok := unicode.IsLetter(r)
		if r < utf8.RuneSelf {
			ok = '0' <= r && r <= '9' || 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || strings.ContainsRune(allowed, r)
		}
		if ok {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// trackPrefix matches the track number file names start with, e.g. "04-".
var trackPrefix = regexp.MustCompile(`^(\d+)[- ]`)

// TagsFile returns the YAML tags file of an mp3 file, or "" if it has none.
// The tags file has the same name as the mp3 file with the .yaml extension.
// If there is no such file, a single tags file in the same folder with the
// same track number prefix is used, so titles may differ in punctuation.
func TagsFile(mp3 string) (string, error) {
	path := strings.TrimSuffix(mp3, filepath.Ext(mp3)) + TagsExt
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	m := trackPrefix.FindStringSubmatch(filepath.Base(mp3))
	if m == nil {
		return "", nil
	}
	candidates, err := filepath.Glob(filepath.Join(filepath.Dir(mp3), m[1]+"*"+TagsExt))
	if err != nil {
		return "", err
	}
	var matches []string
	for _, c := range candidates {
		if cm := trackPrefix.FindStringSubmatch(filepath.Base(c)); cm != nil && cm[1] == m[1] {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 0:
		return "", nil
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%s: ambiguous tags files %s", mp3, strings.Join(matches, ", "))
	}
}

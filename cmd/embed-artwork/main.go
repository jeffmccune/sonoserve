// Command embed-artwork embeds each preset's artwork.jpg into the ID3v2 tag of
// every mp3 file in the preset as the front cover, so Sonos shows the artwork
// while the preset plays.
//
// Tags are written as ID3v2.3 with ISO-8859-1 or UTF-16 text, the most widely
// supported combination. Existing frames are preserved and existing pictures
// are replaced. Files already tagged with the artwork are left untouched.
//
// Usage:
//
//	go run ./cmd/embed-artwork                       # every preset with artwork
//	go run ./cmd/embed-artwork music/presets/9       # one preset directory
//	go run ./cmd/embed-artwork music/presets/9/01-Tulou\ Tagaloa.mp3
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
)

// artworkFilename is the artwork image in each preset folder.
const artworkFilename = "artwork.jpg"

// pictureDescription is the description of the embedded front cover.
const pictureDescription = "Cover"

// v24Renames maps ID3v2.4 date frames to their ID3v2.3 equivalents.
var v24Renames = map[string]string{
	"TDRC": "TYER",
	"TDOR": "TORY",
}

func main() {
	presets := flag.String("presets", "music/presets", "directory containing the preset folders, used when no paths are given")
	dryRun := flag.Bool("n", false, "report what would change without writing any files")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [preset-dir | file.mp3]...\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Embeds each preset's %s into its mp3 files as an ID3v2.3 front cover.\n", artworkFilename)
		fmt.Fprintf(flag.CommandLine.Output(), "With no paths, processes every preset in -presets that has artwork.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	log.SetFlags(0)

	paths := flag.Args()
	if len(paths) == 0 {
		dirs, err := presetDirs(*presets)
		if err != nil {
			log.Fatal(err)
		}
		if len(dirs) == 0 {
			log.Fatalf("no presets in %s have %s", *presets, artworkFilename)
		}
		paths = dirs
	}

	failed := false
	for _, path := range paths {
		if err := process(path, *dryRun); err != nil {
			log.Printf("error: %v", err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

// presetDirs returns the preset directories under root that contain artwork.
func presetDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, artworkFilename)); err == nil {
			dirs = append(dirs, dir)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return dirs, nil
}

// process embeds artwork into a single mp3 file, or into every mp3 file of a
// preset directory.
func process(path string, dryRun bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	dir, files := path, []string(nil)
	if info.IsDir() {
		files, err = filepath.Glob(filepath.Join(dir, "*.mp3"))
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("%s: no mp3 files", dir)
		}
	} else {
		dir, files = filepath.Dir(path), []string{path}
	}

	artwork, err := readArtwork(filepath.Join(dir, artworkFilename))
	if err != nil {
		return err
	}

	var errs []error
	for _, file := range files {
		changed, err := embed(file, artwork, dryRun)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
		case !changed:
			log.Printf("ok       %s", file)
		case dryRun:
			log.Printf("would update %s", file)
		default:
			log.Printf("updated  %s", file)
		}
	}
	return errors.Join(errs...)
}

// readArtwork reads a JPEG image and checks it is baseline encoded, since
// some Sonos controllers fail to show progressive JPEG artwork.
func readArtwork(path string) ([]byte, error) {
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

// embed sets artwork as the only picture in the ID3v2.3 tag of file and
// reports whether the file changed.
func embed(file string, artwork []byte, dryRun bool) (bool, error) {
	tag, err := id3v2.Open(file, id3v2.Options{Parse: true})
	if err != nil {
		return false, err
	}
	defer tag.Close()

	if upToDate(tag, artwork) {
		return false, nil
	}
	if dryRun {
		return true, nil
	}

	if tag.Version() != 3 {
		downgrade(tag)
	}
	tag.DeleteFrames("APIC")
	tag.AddAttachedPicture(id3v2.PictureFrame{
		Encoding:    id3v2.EncodingISO,
		MimeType:    "image/jpeg",
		PictureType: id3v2.PTFrontCover,
		Description: pictureDescription,
		Picture:     artwork,
	})
	return true, tag.Save()
}

// upToDate reports whether tag is ID3v2.3 with artwork as its only picture.
func upToDate(tag *id3v2.Tag, artwork []byte) bool {
	if tag.Version() != 3 {
		return false
	}
	pics := tag.GetFrames("APIC")
	if len(pics) != 1 {
		return false
	}
	pf, ok := pics[0].(id3v2.PictureFrame)
	return ok && pf.PictureType == id3v2.PTFrontCover && bytes.Equal(pf.Picture, artwork)
}

// downgrade converts tag to ID3v2.3. UTF-8 text is not allowed in ID3v2.3, so
// it is re-encoded as ISO-8859-1 or UTF-16, and v2.4 date frames are renamed.
func downgrade(tag *id3v2.Tag) {
	frames := tag.AllFrames()
	tag.SetVersion(3)
	for id, fs := range frames {
		newID := id
		if renamed, ok := v24Renames[id]; ok {
			newID = renamed
		}
		tag.DeleteFrames(id)
		for _, f := range fs {
			switch f := f.(type) {
			case id3v2.TextFrame:
				// ID3v2.4 separates multiple values with NUL, v2.3 with "/".
				f.Text = strings.ReplaceAll(strings.TrimRight(f.Text, "\x00"), "\x00", "/")
				if newID == "TYER" || newID == "TORY" {
					f.Text = year(f.Text)
				}
				f.Encoding = encodingFor(f.Encoding, f.Text)
				tag.AddFrame(newID, f)
			case id3v2.CommentFrame:
				f.Encoding = encodingFor(f.Encoding, f.Description+f.Text)
				tag.AddFrame(newID, f)
			case id3v2.UserDefinedTextFrame:
				f.Encoding = encodingFor(f.Encoding, f.Description+f.Value)
				tag.AddFrame(newID, f)
			case id3v2.UnsynchronisedLyricsFrame:
				f.Encoding = encodingFor(f.Encoding, f.ContentDescriptor+f.Lyrics)
				tag.AddFrame(newID, f)
			case id3v2.PictureFrame:
				f.Encoding = encodingFor(f.Encoding, f.Description)
				tag.AddFrame(newID, f)
			default:
				tag.AddFrame(newID, f)
			}
		}
	}
}

// year returns the leading YYYY of an ID3v2.4 timestamp.
func year(timestamp string) string {
	if len(timestamp) >= 4 {
		return timestamp[:4]
	}
	return timestamp
}

// encodingFor returns an ID3v2.3 compatible replacement for enc: ISO-8859-1
// when text is ASCII, otherwise UTF-16 with BOM.
func encodingFor(enc id3v2.Encoding, text string) id3v2.Encoding {
	if enc.Equals(id3v2.EncodingISO) || enc.Equals(id3v2.EncodingUTF16) {
		return enc
	}
	for _, r := range text {
		if r > 0x7F {
			return id3v2.EncodingUTF16
		}
	}
	return id3v2.EncodingISO
}

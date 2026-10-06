// Command embed-artwork embeds artwork into the ID3v2 tag of every mp3 file
// in a preset as the front cover, so Sonos shows the artwork while the preset
// plays. Each mp3 file uses its own artwork JPEG exported by export-tags if it
// has one, otherwise the preset's artwork.jpg. Files with neither are skipped.
//
// Tags are written as ID3v2.3 with ISO-8859-1 or UTF-16 text, the most widely
// supported combination. Existing frames are preserved and existing pictures
// are replaced. Files already tagged with the artwork are left untouched.
//
// Usage:
//
//	go run ./cmd/embed-artwork                       # every preset
//	go run ./cmd/embed-artwork music/presets/9       # one preset directory
//	go run ./cmd/embed-artwork music/presets/9/01-Tulou\ Tagaloa.mp3
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/jeffmccune/sonoserve/internal/mp3tag"
)

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
		fmt.Fprintf(flag.CommandLine.Output(), "Embeds each mp3 file's artwork JPEG, or the preset's %s, as an ID3v2.3 front cover.\n", mp3tag.ArtworkFilename)
		fmt.Fprintf(flag.CommandLine.Output(), "With no paths, processes every preset in -presets.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	log.SetFlags(0)

	paths := flag.Args()
	if len(paths) == 0 {
		entries, err := os.ReadDir(*presets)
		if err != nil {
			log.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() {
				paths = append(paths, filepath.Join(*presets, e.Name()))
			}
		}
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

// process embeds artwork into a single mp3 file, or into every mp3 file of a
// preset directory.
func process(path string, dryRun bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	files := []string{path}
	if info.IsDir() {
		if files, err = filepath.Glob(filepath.Join(path, "*.mp3")); err != nil {
			return err
		}
	}

	var errs []error
	for _, file := range files {
		_, artworkFile, err := mp3tag.Find(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if artworkFile == "" {
			log.Printf("no art   %s", file)
			continue
		}
		artwork, err := mp3tag.ReadArtwork(artworkFile)
		if err != nil {
			errs = append(errs, err)
			continue
		}
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
	mp3tag.SetArtwork(tag, artwork)
	return true, tag.Save()
}

// upToDate reports whether tag is ID3v2.3 with artwork as its only picture.
func upToDate(tag *id3v2.Tag, artwork []byte) bool {
	return tag.Version() == 3 && mp3tag.HasArtwork(tag, artwork)
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

// encodingFor returns enc if ID3v2.3 allows it, otherwise the ID3v2.3
// encoding for text.
func encodingFor(enc id3v2.Encoding, text string) id3v2.Encoding {
	if enc.Equals(id3v2.EncodingISO) || enc.Equals(id3v2.EncodingUTF16) {
		return enc
	}
	return mp3tag.Encoding(text)
}

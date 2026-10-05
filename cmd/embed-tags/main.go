// Command embed-tags writes the YAML tags exported by export-tags into the
// ID3v2.3 tag of each mp3 file.
//
// The write is authoritative: every existing frame and any ID3v1 tag is
// removed, then the frames in the YAML file are written. The track's artwork
// JPEG exported by export-tags, or else the preset's artwork.jpg, is added as
// the front cover. Without either, the pictures already in the file are kept.
// mp3 files without a YAML tags file
// are left untouched, and files whose tags already match are not rewritten.
//
// Usage:
//
//	go run ./cmd/embed-tags                       # every preset
//	go run ./cmd/embed-tags music/presets/9       # one preset directory
//	go run ./cmd/embed-tags "music/presets/9/08-Shiny.mp3"
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/bogem/id3v2/v2"
	"github.com/jeffmccune/sonoserve/internal/mp3tag"
)

func main() {
	presets := flag.String("presets", "music/presets", "directory containing the preset folders, used when no paths are given")
	dryRun := flag.Bool("n", false, "report what would change without writing any files")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [preset-dir | file.mp3]...\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Replaces the ID3 tags of each mp3 file with its YAML tags file and adds %s.\n", mp3tag.ArtworkFilename)
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

// process embeds tags into a single mp3 file, or into every mp3 file of a
// preset directory.
func process(path string, dryRun bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	dir, files := path, []string(nil)
	if info.IsDir() {
		if files, err = filepath.Glob(filepath.Join(dir, "*.mp3")); err != nil {
			return err
		}
	} else {
		dir, files = filepath.Dir(path), []string{path}
	}

	var errs []error
	used := make(map[string]bool)
	for _, file := range files {
		tagsFile, err := mp3tag.TagsFile(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if tagsFile == "" {
			log.Printf("no tags  %s", file)
			continue
		}
		used[tagsFile] = true
		var artwork []byte
		artworkFile, err := mp3tag.ArtworkFile(file, tagsFile)
		if err == nil && artworkFile != "" {
			artwork, err = mp3tag.ReadArtwork(artworkFile)
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		changed, err := embed(file, tagsFile, artwork, dryRun)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
		case !changed:
			log.Printf("ok       %s", file)
		case dryRun:
			log.Printf("would update %s from %s", file, filepath.Base(tagsFile))
		default:
			log.Printf("updated  %s from %s", file, filepath.Base(tagsFile))
		}
	}

	if info.IsDir() {
		tagsFiles, err := filepath.Glob(filepath.Join(dir, "*"+mp3tag.TagsExt))
		if err != nil {
			return err
		}
		for _, f := range tagsFiles {
			if !used[f] {
				log.Printf("warning: %s matches no mp3 file", f)
			}
		}
	}
	return errors.Join(errs...)
}

// embed replaces the tag of file with the tags in tagsFile plus artwork and
// reports whether the file changed. If artwork is nil, existing pictures are
// kept.
func embed(file, tagsFile string, artwork []byte, dryRun bool) (bool, error) {
	tags, err := mp3tag.ReadTags(tagsFile)
	if err != nil {
		return false, err
	}
	tag, err := id3v2.Open(file, id3v2.Options{Parse: true})
	if err != nil {
		return false, err
	}
	defer tag.Close()

	want := id3v2.NewEmptyTag()
	want.SetVersion(3)
	tags.Apply(want)
	if artwork != nil {
		mp3tag.SetArtwork(want, artwork)
	} else {
		for _, f := range tag.GetFrames("APIC") {
			if pf, ok := f.(id3v2.PictureFrame); ok {
				pf.Encoding = mp3tag.Encoding(pf.Description)
				want.AddAttachedPicture(pf)
			}
		}
	}

	hasV1, err := mp3tag.HasID3v1(file)
	if err != nil {
		return false, err
	}
	if !hasV1 && tag.Version() == 3 && reflect.DeepEqual(tag.AllFrames(), want.AllFrames()) {
		return false, nil
	}
	if dryRun {
		return true, nil
	}

	tag.DeleteAllFrames()
	tag.SetVersion(3)
	for id, frames := range want.AllFrames() {
		for _, f := range frames {
			tag.AddFrame(id, f)
		}
	}
	if err := tag.Save(); err != nil {
		return false, err
	}
	return true, mp3tag.StripID3v1(file)
}

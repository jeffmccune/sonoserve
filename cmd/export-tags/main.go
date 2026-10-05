// Command export-tags reads the tracks of a Music.app playlist and writes the
// metadata of each track to a YAML tags file, which embed-tags writes into
// the ID3 tags of the matching mp3 file.
//
// Tags files are named like the mp3 files Audacity exports with "Numbering
// before Label", e.g. "04-How Far Ill Go.yaml" for the fourth track. The
// playlist is, in order of preference, the one named by -playlist, the one
// named in playlist-name.txt in the -o folder, the playlist currently
// playing, or the playlist shown in the front Music window.
//
// Usage:
//
//	go run ./cmd/export-tags music/presets/9   # uses music/presets/9/playlist-name.txt if it exists
//	go run ./cmd/export-tags -playlist "Moana Live Action Soundtrack" music/presets/9
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jeffmccune/sonoserve/internal/mp3tag"
)

// playlistNameFile names the Music.app playlist of the preset folder it is in.
const playlistNameFile = "playlist-name.txt"

// script is JavaScript for Automation that prints the playlist tracks as JSON.
// Properties are read for all tracks at once, which is much faster than one
// Apple Event per track. Properties a track does not have are null.
const script = `
function run(argv) {
	const music = Application('Music');
	let playlist;
	if (argv.length > 0 && argv[0] !== '') {
		playlist = music.playlists.byName(argv[0]);
	} else {
		try {
			playlist = music.currentPlaylist();
			playlist.name();
		} catch (e) {
			try {
				playlist = music.browserWindows[0].view();
				playlist.name();
			} catch (e) {
				throw new Error('no playlist is playing or shown in the front Music window, use -playlist NAME');
			}
		}
	}
	try {
		playlist.name();
	} catch (e) {
		throw new Error('playlist "' + argv[0] + '" not found');
	}
	const props = ['name', 'artist', 'albumArtist', 'album', 'composer', 'genre',
		'grouping', 'year', 'trackNumber', 'trackCount', 'discNumber', 'discCount',
		'bpm', 'compilation', 'comment', 'lyrics', 'sortName', 'sortArtist',
		'sortAlbumArtist', 'sortAlbum', 'sortComposer', 'duration'];
	const columns = {};
	for (const p of props) {
		try { columns[p] = playlist.tracks[p](); } catch (e) { columns[p] = null; }
	}
	const tracks = [];
	for (let i = 0; i < (columns.name || []).length; i++) {
		const track = {};
		for (const p of props) track[p] = columns[p] ? columns[p][i] : null;
		tracks.push(track);
	}
	return JSON.stringify({playlist: playlist.name(), tracks: tracks});
}
`

// Playlist is the output of script.
type Playlist struct {
	Name   string  `json:"playlist"`
	Tracks []Track `json:"tracks"`
}

// Track is the metadata of one Music.app track.
type Track struct {
	Name            string  `json:"name"`
	Artist          string  `json:"artist"`
	AlbumArtist     string  `json:"albumArtist"`
	Album           string  `json:"album"`
	Composer        string  `json:"composer"`
	Genre           string  `json:"genre"`
	Grouping        string  `json:"grouping"`
	Year            int     `json:"year"`
	TrackNumber     int     `json:"trackNumber"`
	TrackCount      int     `json:"trackCount"`
	DiscNumber      int     `json:"discNumber"`
	DiscCount       int     `json:"discCount"`
	BPM             int     `json:"bpm"`
	Compilation     bool    `json:"compilation"`
	Comment         string  `json:"comment"`
	Lyrics          string  `json:"lyrics"`
	SortName        string  `json:"sortName"`
	SortArtist      string  `json:"sortArtist"`
	SortAlbumArtist string  `json:"sortAlbumArtist"`
	SortAlbum       string  `json:"sortAlbum"`
	SortComposer    string  `json:"sortComposer"`
	Duration        float64 `json:"duration"`
}

// Tags returns the tags of t. Sort fields equal to the field they sort are
// left out since they add nothing.
func (t Track) Tags() *mp3tag.Tags {
	sort := func(sortValue, value string) string {
		if sortValue == value {
			return ""
		}
		return sortValue
	}
	return &mp3tag.Tags{
		Title:           t.Name,
		Artist:          t.Artist,
		AlbumArtist:     t.AlbumArtist,
		Album:           t.Album,
		Composer:        t.Composer,
		Genre:           t.Genre,
		Grouping:        t.Grouping,
		Year:            t.Year,
		Track:           t.TrackNumber,
		TrackCount:      t.TrackCount,
		Disc:            t.DiscNumber,
		DiscCount:       t.DiscCount,
		BPM:             t.BPM,
		Compilation:     t.Compilation,
		Comment:         t.Comment,
		Lyrics:          t.Lyrics,
		SortTitle:       sort(t.SortName, t.Name),
		SortArtist:      sort(t.SortArtist, t.Artist),
		SortAlbumArtist: sort(t.SortAlbumArtist, t.AlbumArtist),
		SortAlbum:       sort(t.SortAlbum, t.Album),
		SortComposer:    sort(t.SortComposer, t.Composer),
	}
}

func main() {
	playlistName := flag.String("playlist", "", "name of the Music.app playlist (default: the one in "+playlistNameFile+" in the -o folder, the current playlist, or the one in the front window)")
	outDir := flag.String("o", ".", "directory to write the YAML tags files to, usually the preset folder (or pass it as an argument)")
	force := flag.Bool("f", false, "overwrite existing tags files")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [preset-dir]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Writes a YAML tags file for each track of a Music.app playlist.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	switch flag.NArg() {
	case 0:
	case 1:
		*outDir = flag.Arg(0)
	default:
		flag.Usage()
		os.Exit(2)
	}
	log.SetFlags(0)

	name := *playlistName
	if name == "" {
		var err error
		if name, err = readPlaylistName(filepath.Join(*outDir, playlistNameFile)); err != nil {
			log.Fatal(err)
		}
	}

	playlist, err := getPlaylist(name)
	if err != nil {
		log.Fatal(err)
	}
	if len(playlist.Tracks) == 0 {
		log.Fatalf("playlist %q has no tracks", playlist.Name)
	}
	log.Printf("Playlist %q has %d tracks", playlist.Name, len(playlist.Tracks))

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	width := max(2, len(fmt.Sprint(len(playlist.Tracks))))
	for i, track := range playlist.Tracks {
		name := fmt.Sprintf("%0*d-%s%s", width, i+1, mp3tag.FileName(track.Name), mp3tag.TagsExt)
		path := filepath.Join(*outDir, name)
		if _, err := os.Stat(path); err == nil && !*force {
			log.Printf("exists   %s (use -f to overwrite)", path)
			continue
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Fatal(err)
		}
		header := fmt.Sprintf("Exported by export-tags from Music.app playlist %q, track %d of %d.\n"+
			"embed-tags writes these tags into the mp3 file with the same name or track number.",
			playlist.Name, i+1, len(playlist.Tracks))
		data, err := track.Tags().Marshal(header)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote    %s", path)
	}
}

// readPlaylistName returns the playlist name in path, or "" if path does not
// exist.
func readPlaylistName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", fmt.Errorf("%s is empty", path)
	}
	log.Printf("Using playlist %q from %s", name, path)
	return name, nil
}

// getPlaylist returns the tracks of the named playlist, or the current one
// if name is empty.
func getPlaylist(name string) (*Playlist, error) {
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", script, name)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("osascript: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	var playlist Playlist
	if err := json.Unmarshal(out, &playlist); err != nil {
		return nil, fmt.Errorf("parsing osascript output: %w", err)
	}
	return &playlist, nil
}

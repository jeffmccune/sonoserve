---
sidebar_position: 5
---

# MP3 Tags

Sonos shows the title, artist, album, and artwork from the ID3 tags of each
mp3 file. Preset mp3 files are recorded from Music.app with Audacity (see
`tracks/readme.md`), so their tags start out incomplete. Three commands fix
them:

| Command | Reads | Writes |
|---|---|---|
| `export-tags` | Music.app playlist | A YAML tags file per track |
| `embed-tags` | YAML tags files and `artwork.jpg` | The full ID3 tag, replacing what is there |
| `embed-artwork` | `artwork.jpg` | The cover picture only, keeping the other tags |

All three write ID3v2.3 tags with ISO-8859-1 or UTF-16 text, the most widely
supported combination. ID3v2.4 and UTF-8 text are avoided.

## Workflow

1. Record the playlist and export the mp3 files into the preset folder, e.g.
   `music/presets/9`, as described in `tracks/readme.md`. Remove any
   apostrophes from the file names, since `go:embed` rejects them.
2. Add the preset's `artwork.jpg`, a baseline (not progressive) JPEG.
3. Put the name of the Music.app playlist in `playlist-name.txt` in the preset
   folder, then export its tags:

   ```bash
   echo "Moana Live Action Soundtrack" > music/presets/9/playlist-name.txt
   go run ./cmd/export-tags -o music/presets/9
   ```

4. Review and edit the YAML files. Music.app metadata is sometimes wrong, for
   example the year or the track count.
5. Write the tags into the mp3 files:

   ```bash
   go run ./cmd/embed-tags music/presets/9
   ```

6. Commit the YAML files. The mp3 files are git-ignored, so the YAML files are
   the record of their tags and `embed-tags` can rewrite them at any time.

Repeat step 5 after editing a YAML file or the artwork. Each command can be
re-run safely: files that already match are reported as `ok` and not
rewritten.

## export-tags

`export-tags` runs JavaScript for Automation through `osascript`, like the
program in the `tracks` folder, and writes one YAML file per track to the `-o`
directory (default `.`).

| Flag | Default | Description |
|---|---|---|
| `-playlist` | | Name of the Music.app playlist, overrides `playlist-name.txt` |
| `-o` | `.` | Directory to write the YAML files to, usually the preset folder |
| `-f` | `false` | Overwrite existing YAML files, which are kept by default so hand edits are not lost |

The playlist is, in order of preference:

1. The `-playlist` flag.
2. The contents of `playlist-name.txt` in the `-o` folder, if the file exists.
   Leading and trailing whitespace is ignored, and an empty file is an error.
3. The playlist that is currently playing.
4. The playlist shown in the front Music window.

Naming the playlist with the flag or `playlist-name.txt` is the most reliable,
because the last two depend on what Music.app is doing. Commit
`playlist-name.txt` with the preset so later exports read the same playlist.
The playlist name is recorded in the comment at the top of each YAML file.

Files are named like the mp3 files Audacity exports with "Numbering before
Label": the position in the playlist, a dash, and the track name without the
characters `go:embed` rejects. The fourth track, "How Far I'll Go", becomes
`04-How Far Ill Go.yaml`, matching `04-How Far Ill Go.mp3`.

```yaml
# Exported by export-tags from Music.app playlist "Moana Live Action Soundtrack", track 1 of 13.
# embed-tags writes these tags into the mp3 file with the same name or track number.
title: Tulou Tagaloa
artist: Olivia Foa'i, Opetaia Foa’i, Matatia Foai & Disney
album_artist: Lin-Manuel Miranda, Catherine Laga'aia & Dwayne Johnson
album: Moana (Original Motion Picture Soundtrack)
composer: Opetaia Foa’i
genre: Soundtrack
year: 2016
track: 1
track_count: 12
disc: 1
disc_count: 1
```

Fields Music.app leaves empty are omitted. Sort fields are only written when
they differ from the field they sort.

### Fields

Each field maps to one ID3v2.3 frame.

| Field | Frame | Notes |
|---|---|---|
| `title` | `TIT2` | |
| `artist` | `TPE1` | |
| `album_artist` | `TPE2` | |
| `album` | `TALB` | |
| `composer` | `TCOM` | |
| `genre` | `TCON` | |
| `grouping` | `TIT1` | |
| `year` | `TYER` | |
| `track`, `track_count` | `TRCK` | Written as `7/13`, or `7` without a count |
| `disc`, `disc_count` | `TPOS` | Written as `1/1`, or `1` without a count |
| `bpm` | `TBPM` | |
| `compilation` | `TCMP` | `1` when true, the iTunes convention |
| `comment` | `COMM` | Language `eng`, empty description |
| `lyrics` | `USLT` | Language `eng` |
| `sort_title` | `TSOT` | |
| `sort_artist` | `TSOP` | |
| `sort_album_artist` | `TSO2` | |
| `sort_album` | `TSOA` | |
| `sort_composer` | `TSOC` | |

Unknown fields are an error, so a typo such as `titel:` is caught.

## embed-tags

```bash
go run ./cmd/embed-tags [-n] [-presets music/presets] [preset-dir | file.mp3]...
```

With no paths, it processes every folder in `-presets`. `-n` reports what
would change without writing anything.

For each mp3 file, `embed-tags` looks for its YAML file:

1. The file with the same name and a `.yaml` extension.
2. Otherwise, the only YAML file in the folder with the same track number
   prefix, e.g. `04-` or `04 `. Titles can then differ in punctuation.
   More than one match is an error.

mp3 files without a YAML file are reported as `no tags` and left untouched.
YAML files that match no mp3 file are reported as warnings.

The write is authoritative. Every ID3v2 frame is removed, including encoder
and comment frames left by Audacity, and any ID3v1 tag at the end of the file
is stripped. Only the frames from the YAML file are written. The artwork is
then added on top:

- If the folder has an `artwork.jpg`, it becomes the only picture, the front
  cover.
- Otherwise, the pictures already in the file are kept.

The audio is not re-encoded or changed. The new file is written next to the
original and renamed over it, so an interrupted write leaves the original
intact.

## embed-artwork

```bash
go run ./cmd/embed-artwork [-n] [-presets music/presets] [preset-dir | file.mp3]...
```

`embed-artwork` only replaces the pictures with `artwork.jpg` and keeps every
other frame. It is useful for presets without YAML files. With no paths, it
processes every preset folder that has an `artwork.jpg`. ID3v2.4 tags are
converted to ID3v2.3 along the way. UTF-8 text is re-encoded, and `TDRC` and
`TDOR` become `TYER` and `TORY`.

`embed-tags` already adds the artwork, so there is no need to run both.

## Implementation

The shared code is in `internal/mp3tag`: the YAML schema (`Tags`), the
mapping to frames (`Tags.Apply`), artwork checks, ID3v1 removal, and file
matching. Tags are read and written with
[github.com/bogem/id3v2](https://github.com/bogem/id3v2).

To decide whether a file is up to date, `embed-tags` builds the wanted tag in
memory and compares its frames with the frames parsed from the file. A file is
rewritten only if they differ, the file is not ID3v2.3, or it has an ID3v1
tag.

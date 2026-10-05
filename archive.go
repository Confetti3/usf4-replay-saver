package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	replayExt = ".usf4replay"
	// entryDirName holds the game's index entry for each replay, by CRC.
	entryDirName = ".index"
)

var archivedName = regexp.MustCompile(`_([0-9a-f]{8})\` + replayExt + `$`)

type archive struct {
	dir   string
	mu    sync.Mutex
	known map[uint32]string // CRC to file name
}

func openArchive(dir string) (*archive, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	a := &archive{dir: dir, known: map[uint32]string{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		m := archivedName.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		crc, err := strconv.ParseUint(m[1], 16, 32)
		if err == nil {
			a.known[uint32(crc)] = entry.Name()
		}
	}
	return a, nil
}

func (a *archive) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.known)
}

// save copies a replay into the archive unless an identical one is there.
// It returns the file name and whether it wrote a new file.
func (a *archive) save(s *slot) (string, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if name, ok := a.known[s.crc]; ok {
		return name, false, nil
	}
	when := s.written
	if when.IsZero() {
		when = time.Now()
	}
	name := fmt.Sprintf("%s_%08x%s", when.Local().Format("2006-01-02_15-04-05"), s.crc, replayExt)
	final := filepath.Join(a.dir, name)
	tmp := final + ".part"
	if err := os.WriteFile(tmp, s.data, 0o644); err != nil {
		return "", false, err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", false, err
	}
	if !when.IsZero() {
		os.Chtimes(final, when, when)
	}
	a.known[s.crc] = name
	return name, true, nil
}

type scanResult struct {
	saved   []string
	entries int // index entries kept, including ones for replays saved earlier
	waiting int // slots skipped because their checksum did not match yet
}

// scan archives every finished replay in the given slot range, with the
// game's index entry for it so it can be put back later.
func (a *archive) scan(dirs []string, first, last int) (scanResult, error) {
	var result scanResult
	for _, dir := range dirs {
		ring, _ := readRingIndex(dir)
		var saved *replayIndex
		if first < ringFirst {
			saved, _ = readSavedIndex(dir)
		}
		for n := first; n <= last; n++ {
			s, err := readSlot(dir, n)
			if err != nil {
				if err == errChecksum {
					result.waiting++
				}
				continue
			}
			name, isNew, err := a.save(s)
			if err != nil {
				return result, fmt.Errorf("saving slot %d: %w", n, err)
			}
			if isNew {
				result.saved = append(result.saved, name)
			}
			// The game rewrites the index just after the replay, so an entry
			// that does not match yet is picked up on a later scan.
			idx := ring
			if n < ringFirst {
				idx = saved
			}
			if idx != nil && !a.hasEntry(s.crc) {
				if e := idx.entry(n, s.crc); e != nil && validEntry(e, s.crc, len(s.data)) {
					if err := a.saveEntry(s.crc, e); err != nil {
						return result, fmt.Errorf("saving the index entry for slot %d: %w", n, err)
					}
					result.entries++
				}
			}
		}
	}
	return result, nil
}

func (a *archive) entryPath(crc uint32) string {
	return filepath.Join(a.dir, entryDirName, fmt.Sprintf("%08x.entry", crc))
}

func (a *archive) hasEntry(crc uint32) bool {
	_, err := os.Stat(a.entryPath(crc))
	return err == nil
}

func (a *archive) loadEntry(crc uint32) []byte {
	e, err := os.ReadFile(a.entryPath(crc))
	if err != nil || len(e) != entrySize {
		return nil
	}
	return e
}

func (a *archive) saveEntry(crc uint32, e []byte) error {
	dir := filepath.Join(a.dir, entryDirName)
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		hideFile(dir)
	}
	path := a.entryPath(crc)
	if err := os.WriteFile(path+".part", e, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".part", path)
}

func listArchive(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), replayExt) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		status := "ok"
		if err != nil {
			status = err.Error()
		} else if err := validateReplay(data); err != nil {
			status = "not a USF4 replay"
		} else if m := archivedName.FindStringSubmatch(entry.Name()); m != nil && fmt.Sprintf("%08x", crc32.ChecksumIEEE(data)) != m[1] {
			status = "damaged (checksum differs from the name)"
		}
		fmt.Printf("  %s  %s\n", entry.Name(), status)
		count++
	}
	fmt.Printf("%d replays in %s\n", count, dir)
	return nil
}

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
)

var (
	errGameRunning = errors.New("close Street Fighter IV first; the game rewrites its save files while it runs")
	errNoEntry     = errors.New("this replay was saved without the game's list details, so the game cannot show it; replays saved from now on can be put back")
)

// restore puts an archived replay back into the game's recent-match ring the
// way the game writes one after a match: the replay file, its checksum, and
// its entry in the ring index the menus read. It replaces the ring slot with
// the oldest match, after archiving that one. The game must be closed. It
// returns the slot written, or -1 when the replay was already there.
func restore(file string, dir string, a *archive) (int, error) {
	if running, err := gameRunning(); err == nil && running {
		return 0, errGameRunning
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	if err := validateReplay(data); err != nil {
		return 0, err
	}
	crc := crc32.ChecksumIEEE(data)
	entry := a.loadEntry(crc)
	if !validEntry(entry, crc, len(data)) {
		return 0, errNoEntry
	}
	ring, err := readRingIndex(dir)
	if err != nil {
		return 0, fmt.Errorf("the game's list of recent replays could not be read: %w", err)
	}

	target := ringTarget(dir, ring, crc)
	if target < 0 {
		return -1, nil
	}
	if s, err := readSlot(dir, target); err == nil {
		if _, _, err := a.save(s); err != nil {
			return 0, fmt.Errorf("could not save slot %d before replacing it: %w", target, err)
		}
		if e := ring.entry(target, s.crc); e != nil && !a.hasEntry(s.crc) {
			if err := a.saveEntry(s.crc, e); err != nil {
				return 0, fmt.Errorf("could not save slot %d's details before replacing it: %w", target, err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) && err != errNotReplay && err != errChecksum {
		return 0, fmt.Errorf("slot %d could not be read, so it was left alone: %w", target, err)
	}

	path := filepath.Join(dir, strconv.Itoa(target))
	if err := writeReplaced(path, data); err != nil {
		return 0, err
	}
	sum := make([]byte, 4)
	binary.LittleEndian.PutUint32(sum, crc)
	if err := writeReplaced(path+".0", sum); err != nil {
		return 0, err
	}
	if err := ring.put(target, entry); err != nil {
		return 0, fmt.Errorf("the replay was written but the game's list was not updated: %w", err)
	}
	return target, nil
}

// ringTarget picks the ring slot to write. It returns -1 when the replay and
// its entry are already in place. A slot holding the replay without a
// matching entry (left by an older version of this app) is repaired in
// place. Otherwise an empty or inconsistent slot comes first, then the slot
// whose entry has the oldest time.
func ringTarget(dir string, ring *replayIndex, crc uint32) int {
	best, bestTime, bestEmpty := -1, uint32(0), false
	for n := ringFirst; n <= ringLast; n++ {
		s, err := readSlot(dir, n)
		var e []byte
		if err == nil {
			e = ring.entry(n, s.crc)
		}
		if err == nil && s.crc == crc {
			if e != nil {
				return -1
			}
			return n
		}
		if err != nil || e == nil {
			if !bestEmpty {
				best, bestEmpty = n, true
			}
			continue
		}
		if bestEmpty {
			continue
		}
		if t := binary.LittleEndian.Uint32(e[9:]); best < 0 || t < bestTime {
			best, bestTime = n, t
		}
	}
	return best
}

func writeReplaced(path string, data []byte) error {
	tmp := path + ".restore-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
